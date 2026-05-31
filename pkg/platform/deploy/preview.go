package deploy

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

const RequiredPulumiVersion = ">=3.244.0"

type PreviewOptions struct {
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

	runPreviewFn previewRunner
}

type PreviewResult struct {
	StackName     string
	ChangeSummary map[string]int
}

type previewRunner func(context.Context, automationProgramOptions) (PreviewResult, error)

func PreviewCluster(ctx context.Context, opts PreviewOptions) (PreviewResult, error) {
	if opts.ProjectName == "" {
		return PreviewResult{}, fmt.Errorf("project name is required")
	}
	if opts.StackName == "" {
		return PreviewResult{}, fmt.Errorf("stack name is required")
	}
	if opts.WorkDir == "" {
		return PreviewResult{}, fmt.Errorf("Pulumi work directory is required")
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
		return PreviewResult{}, err
	}

	runner := opts.runPreviewFn
	if runner == nil {
		runner = runAutomationPreview
	}

	return runner(ctx, automationProgramOptions{
		ProjectName:            opts.ProjectName,
		StackName:              opts.StackName,
		WorkDir:                opts.WorkDir,
		EnvVars:                copyStringMap(opts.EnvVars),
		HCloudToken:            opts.EnvVars["HCLOUD_TOKEN"],
		PulumiConfigPassphrase: opts.EnvVars["PULUMI_CONFIG_PASSPHRASE"],
		Environment:            prepared.Environment,
	})
}

func runAutomationPreview(ctx context.Context, opts automationProgramOptions) (PreviewResult, error) {
	if err := os.MkdirAll(opts.WorkDir, 0o700); err != nil {
		return PreviewResult{}, fmt.Errorf("create Pulumi work directory: %w", err)
	}
	envVars, err := prepareAutomationEnv(opts.WorkDir, opts.EnvVars)
	if err != nil {
		return PreviewResult{}, err
	}

	stack, err := auto.UpsertStackInlineSource(
		ctx,
		opts.StackName,
		opts.ProjectName,
		PulumiProgram(opts.Environment, opts.ImageRefs, opts.HCloudToken, opts.PulumiConfigPassphrase),
		stackOptions(opts.ProjectName, opts.WorkDir, envVars)...,
	)
	if err != nil {
		return PreviewResult{}, err
	}

	result, err := stack.Preview(ctx, optpreview.SuppressProgress())
	if err != nil {
		return PreviewResult{}, err
	}

	return PreviewResult{
		StackName:     opts.StackName,
		ChangeSummary: stringifyChangeSummary(result.ChangeSummary),
	}, nil
}

func stringifyChangeSummary(summary map[apitype.OpType]int) map[string]int {
	values := make(map[string]int, len(summary))
	for op, count := range summary {
		values[string(op)] = count
	}

	return values
}

func copyStringMap(values map[string]string) map[string]string {
	copied := make(map[string]string, len(values))
	for name, value := range values {
		copied[name] = value
	}

	return copied
}
