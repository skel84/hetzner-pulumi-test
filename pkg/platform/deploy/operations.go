package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type UpOptions struct {
	ProjectName        string
	StackName          string
	WorkDir            string
	ConfigPath         string
	EnvironmentName    string
	ControlPlaneCount  int
	WorkerCount        int
	WorkerCountSet     bool
	CurrentIP          string
	ResolveCurrentIPFn CurrentIPResolver
	EnvVars            map[string]string
	Stdout             io.Writer
	Stderr             io.Writer

	EnsureImagesFn ImageEnsurer
	runUpFn        upRunner
}

type DestroyOptions struct {
	ProjectName string
	StackName   string
	WorkDir     string
	EnvVars     map[string]string
	Stdout      io.Writer
	Stderr      io.Writer

	runDestroyFn destroyRunner
}

type UpResult struct {
	StackName      string
	ChangeSummary  map[string]int
	ImagesCreated  []string
	ImagesExisting []string
}

type DestroyResult struct {
	StackName     string
	ChangeSummary map[string]int
}

type upRunner func(context.Context, automationProgramOptions) (UpResult, error)
type destroyRunner func(context.Context, automationStackOptions) (DestroyResult, error)

type automationProgramOptions struct {
	ProjectName string
	StackName   string
	WorkDir     string
	EnvVars     map[string]string
	HCloudToken string
	Environment config.EnvironmentSpec
	ImageRefs   map[string]string
	Stdout      io.Writer
	Stderr      io.Writer
}

type automationStackOptions struct {
	ProjectName string
	StackName   string
	WorkDir     string
	EnvVars     map[string]string
	Stdout      io.Writer
	Stderr      io.Writer
}

func UpCluster(ctx context.Context, opts UpOptions) (UpResult, error) {
	if opts.ProjectName == "" {
		return UpResult{}, fmt.Errorf("project name is required")
	}
	if opts.StackName == "" {
		return UpResult{}, fmt.Errorf("stack name is required")
	}
	if opts.WorkDir == "" {
		return UpResult{}, fmt.Errorf("Pulumi work directory is required")
	}

	prepared, err := PrepareEnvironment(ctx, PrepareOptions{
		ConfigPath:         opts.ConfigPath,
		EnvironmentName:    opts.EnvironmentName,
		ControlPlaneCount:  opts.ControlPlaneCount,
		WorkerCount:        opts.WorkerCount,
		WorkerCountSet:     opts.WorkerCountSet,
		CurrentIP:          opts.CurrentIP,
		ResolveCurrentIPFn: opts.ResolveCurrentIPFn,
	})
	if err != nil {
		return UpResult{}, err
	}
	imageEnsurer := opts.EnsureImagesFn
	if imageEnsurer == nil {
		imageEnsurer = EnsureHCloudImages
	}
	writeProgress(opts.Stdout, "Ensuring Talos images...\n")
	images, err := imageEnsurer(ctx, opts.EnvVars["HCLOUD_TOKEN"], RequiredTalosImageSpecs(prepared.Environment))
	if err != nil {
		return UpResult{}, err
	}
	writeImageEnsureProgress(opts.Stdout, images)

	runner := opts.runUpFn
	if runner == nil {
		runner = runAutomationUp
	}

	result, err := runner(ctx, automationProgramOptions{
		ProjectName: opts.ProjectName,
		StackName:   opts.StackName,
		WorkDir:     opts.WorkDir,
		EnvVars:     copyStringMap(opts.EnvVars),
		HCloudToken: opts.EnvVars["HCLOUD_TOKEN"],
		Environment: prepared.Environment,
		ImageRefs:   copyStringMap(images.Refs),
		Stdout:      opts.Stdout,
		Stderr:      opts.Stderr,
	})
	if err != nil {
		return UpResult{}, err
	}
	result.ImagesCreated = append([]string{}, images.Created...)
	result.ImagesExisting = append([]string{}, images.Existing...)

	return result, nil
}

func DestroyCluster(ctx context.Context, opts DestroyOptions) (DestroyResult, error) {
	if opts.ProjectName == "" {
		return DestroyResult{}, fmt.Errorf("project name is required")
	}
	if opts.StackName == "" {
		return DestroyResult{}, fmt.Errorf("stack name is required")
	}
	if opts.WorkDir == "" {
		return DestroyResult{}, fmt.Errorf("Pulumi work directory is required")
	}

	runner := opts.runDestroyFn
	if runner == nil {
		runner = runAutomationDestroy
	}

	return runner(ctx, automationStackOptions{
		ProjectName: opts.ProjectName,
		StackName:   opts.StackName,
		WorkDir:     opts.WorkDir,
		EnvVars:     copyStringMap(opts.EnvVars),
		Stdout:      opts.Stdout,
		Stderr:      opts.Stderr,
	})
}

func runAutomationUp(ctx context.Context, opts automationProgramOptions) (UpResult, error) {
	if err := os.MkdirAll(opts.WorkDir, 0o700); err != nil {
		return UpResult{}, fmt.Errorf("create Pulumi work directory: %w", err)
	}
	envVars, err := prepareAutomationEnv(opts.WorkDir, opts.EnvVars)
	if err != nil {
		return UpResult{}, err
	}

	stack, err := auto.UpsertStackInlineSource(
		ctx,
		opts.StackName,
		opts.ProjectName,
		PulumiProgram(opts.Environment, opts.ImageRefs, opts.HCloudToken),
		stackOptions(opts.ProjectName, opts.WorkDir, envVars)...,
	)
	if err != nil {
		return UpResult{}, err
	}

	result, err := stack.Up(ctx, upOptions(opts)...)
	if err != nil {
		return UpResult{}, automationError(err, opts.Stdout != nil || opts.Stderr != nil)
	}

	return UpResult{
		StackName:     opts.StackName,
		ChangeSummary: changeSummaryFromUpdate(result.Summary.ResourceChanges),
	}, nil
}

func runAutomationDestroy(ctx context.Context, opts automationStackOptions) (DestroyResult, error) {
	if err := os.MkdirAll(opts.WorkDir, 0o700); err != nil {
		return DestroyResult{}, fmt.Errorf("create Pulumi work directory: %w", err)
	}
	envVars, err := prepareAutomationEnv(opts.WorkDir, opts.EnvVars)
	if err != nil {
		return DestroyResult{}, err
	}

	stack, err := auto.SelectStackInlineSource(
		ctx,
		opts.StackName,
		opts.ProjectName,
		func(*pulumi.Context) error { return nil },
		stackOptions(opts.ProjectName, opts.WorkDir, envVars)...,
	)
	if err != nil {
		return DestroyResult{}, err
	}

	result, err := stack.Destroy(ctx, destroyOptions(opts)...)
	if err != nil {
		return DestroyResult{}, automationError(err, opts.Stdout != nil || opts.Stderr != nil)
	}

	return DestroyResult{
		StackName:     opts.StackName,
		ChangeSummary: changeSummaryFromUpdate(result.Summary.ResourceChanges),
	}, nil
}

func stackOptions(projectName string, workDir string, envVars map[string]string) []auto.LocalWorkspaceOption {
	project := workspace.Project{
		Name:                  tokens.PackageName(projectName),
		Runtime:               workspace.NewProjectRuntimeInfo("go", nil),
		RequiredPulumiVersion: RequiredPulumiVersion,
	}

	return []auto.LocalWorkspaceOption{
		auto.Project(project),
		auto.WorkDir(workDir),
		auto.EnvVars(copyStringMap(envVars)),
		auto.SecretsProvider("passphrase"),
	}
}

func prepareAutomationEnv(workDir string, envVars map[string]string) (map[string]string, error) {
	prepared := copyStringMap(envVars)
	helmDir := filepath.Join(workDir, "helm")
	repositoryCache := filepath.Join(helmDir, "repository-cache")
	configHome := filepath.Join(helmDir, "config")
	cacheHome := filepath.Join(helmDir, "cache")
	dataHome := filepath.Join(helmDir, "data")
	for _, path := range []string{repositoryCache, configHome, cacheHome, dataHome} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, fmt.Errorf("create Helm workspace directory: %w", err)
		}
	}

	repositoryConfig := filepath.Join(helmDir, "repositories.yaml")
	registryConfig := filepath.Join(helmDir, "registry.json")
	prepared["PULUMI_K8S_HELM_REPOSITORY_CONFIG_PATH"] = repositoryConfig
	prepared["PULUMI_K8S_HELM_REPOSITORY_CACHE"] = repositoryCache
	prepared["PULUMI_K8S_HELM_REGISTRY_CONFIG_PATH"] = registryConfig
	prepared["HELM_REPOSITORY_CONFIG"] = repositoryConfig
	prepared["HELM_REPOSITORY_CACHE"] = repositoryCache
	prepared["HELM_REGISTRY_CONFIG"] = registryConfig
	prepared["HELM_CONFIG_HOME"] = configHome
	prepared["HELM_CACHE_HOME"] = cacheHome
	prepared["HELM_DATA_HOME"] = dataHome

	return prepared, nil
}

func changeSummaryFromUpdate(summary *map[string]int) map[string]int {
	if summary == nil {
		return map[string]int{}
	}

	values := make(map[string]int, len(*summary))
	for op, count := range *summary {
		values[op] = count
	}

	return values
}

func upOptions(opts automationProgramOptions) []optup.Option {
	options := []optup.Option{
		optup.ShowSecrets(false),
	}
	if opts.Stdout != nil {
		options = append(options, optup.ProgressStreams(opts.Stdout))
	} else {
		options = append(options, optup.SuppressProgress())
	}
	if opts.Stderr != nil {
		options = append(options, optup.ErrorProgressStreams(opts.Stderr))
	}

	return options
}

func destroyOptions(opts automationStackOptions) []optdestroy.Option {
	options := []optdestroy.Option{
		optdestroy.RunProgram(false),
		optdestroy.ShowSecrets(false),
	}
	if opts.Stdout != nil {
		options = append(options, optdestroy.ProgressStreams(opts.Stdout))
	} else {
		options = append(options, optdestroy.SuppressProgress())
	}
	if opts.Stderr != nil {
		options = append(options, optdestroy.ErrorProgressStreams(opts.Stderr))
	}

	return options
}

func writeImageEnsureProgress(writer io.Writer, result ImageEnsureResult) {
	if writer == nil {
		return
	}

	for _, image := range result.Existing {
		writeProgress(writer, "Talos image reused: %s\n", image)
	}
	for _, image := range result.Created {
		writeProgress(writer, "Talos image created: %s\n", image)
	}
}

func writeProgress(writer io.Writer, format string, args ...any) {
	if writer == nil {
		return
	}
	fmt.Fprintf(writer, format, args...)
}

func automationError(err error, streamed bool) error {
	if !streamed {
		return err
	}

	message := err.Error()
	firstLine := message
	if index := strings.IndexByte(message, '\n'); index >= 0 {
		firstLine = message[:index]
	}

	code := ""
	for _, line := range strings.Split(message, "\n") {
		if strings.HasPrefix(line, "code: ") {
			code = strings.TrimSpace(strings.TrimPrefix(line, "code: "))
			break
		}
	}
	if code == "" {
		return fmt.Errorf("%s", firstLine)
	}

	return fmt.Errorf("%s (exit code %s)", firstLine, code)
}
