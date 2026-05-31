package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/francesco/hetzner_pulumi/pkg/platform/deploy"
)

const (
	DefaultEnvironmentName    = "dev"
	DefaultConfigPath         = "config/environments.yaml"
	DefaultPulumiProjectName  = "hetzner-pulumi-cluster"
	defaultPulumiWorkspaceDir = ".pulumi/platformctl/cluster"
)

type Previewer interface {
	PreviewCluster(context.Context, deploy.PreviewOptions) (deploy.PreviewResult, error)
	UpCluster(context.Context, deploy.UpOptions) (deploy.UpResult, error)
	DestroyCluster(context.Context, deploy.DestroyOptions) (deploy.DestroyResult, error)
	ReadClusterOutput(context.Context, deploy.OutputOptions) (deploy.OutputResult, error)
	DoctorCluster(context.Context, deploy.DoctorOptions) (deploy.DoctorResult, error)
}

type PreviewerFunc func(context.Context, deploy.PreviewOptions) (deploy.PreviewResult, error)

func (f PreviewerFunc) PreviewCluster(ctx context.Context, opts deploy.PreviewOptions) (deploy.PreviewResult, error) {
	return f(ctx, opts)
}

func (f PreviewerFunc) UpCluster(context.Context, deploy.UpOptions) (deploy.UpResult, error) {
	return deploy.UpResult{}, fmt.Errorf("up is not implemented by this previewer")
}

func (f PreviewerFunc) DestroyCluster(context.Context, deploy.DestroyOptions) (deploy.DestroyResult, error) {
	return deploy.DestroyResult{}, fmt.Errorf("destroy is not implemented by this previewer")
}

func (f PreviewerFunc) ReadClusterOutput(context.Context, deploy.OutputOptions) (deploy.OutputResult, error) {
	return deploy.OutputResult{}, fmt.Errorf("output retrieval is not implemented by this previewer")
}

func (f PreviewerFunc) DoctorCluster(context.Context, deploy.DoctorOptions) (deploy.DoctorResult, error) {
	return deploy.DoctorResult{}, fmt.Errorf("doctor is not implemented by this previewer")
}

type defaultClusterOperator struct{}

func (defaultClusterOperator) PreviewCluster(ctx context.Context, opts deploy.PreviewOptions) (deploy.PreviewResult, error) {
	return deploy.PreviewCluster(ctx, opts)
}

func (defaultClusterOperator) UpCluster(ctx context.Context, opts deploy.UpOptions) (deploy.UpResult, error) {
	return deploy.UpCluster(ctx, opts)
}

func (defaultClusterOperator) DestroyCluster(ctx context.Context, opts deploy.DestroyOptions) (deploy.DestroyResult, error) {
	return deploy.DestroyCluster(ctx, opts)
}

func (defaultClusterOperator) ReadClusterOutput(ctx context.Context, opts deploy.OutputOptions) (deploy.OutputResult, error) {
	return deploy.ReadClusterOutput(ctx, opts)
}

func (defaultClusterOperator) DoctorCluster(ctx context.Context, opts deploy.DoctorOptions) (deploy.DoctorResult, error) {
	return deploy.DoctorCluster(ctx, opts)
}

type CommandOptions struct {
	Stdout    io.Writer
	Stderr    io.Writer
	Getenv    func(string) string
	WorkDir   string
	Previewer Previewer
}

type Command struct {
	stdout    io.Writer
	stderr    io.Writer
	getenv    func(string) string
	workDir   string
	previewer Previewer
}

func New(opts CommandOptions) *Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	workDir := opts.WorkDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			workDir = "."
		}
	}
	previewer := opts.Previewer
	if previewer == nil {
		previewer = defaultClusterOperator{}
	}

	return &Command{
		stdout:    stdout,
		stderr:    stderr,
		getenv:    getenv,
		workDir:   workDir,
		previewer: previewer,
	}
}

func (c *Command) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		c.printUsage()
		return 2
	}

	switch args[0] {
	case "cluster":
		return c.runCluster(ctx, args[1:])
	case "up":
		return c.runTopLevelClusterAction(ctx, "up", args[1:])
	case "down":
		return c.runTopLevelClusterAction(ctx, "down", args[1:])
	case "kubeconfig":
		return c.runTopLevelOutput(ctx, "kubeconfig", args[1:])
	case "talosconfig":
		return c.runTopLevelOutput(ctx, "talosconfig", args[1:])
	case "doctor":
		return c.runTopLevelDoctor(ctx, args[1:])
	default:
		fmt.Fprintf(c.stderr, "platformctl: unknown command %q\n", args[0])
		c.printUsage()
		return 2
	}
}

func (c *Command) runCluster(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, "platformctl cluster: subcommand is required")
		c.printClusterUsage()
		return 2
	}

	switch args[0] {
	case "preview":
		return c.runClusterPreview(ctx, args[1:])
	case "up":
		return c.runClusterUp(ctx, args[1:])
	case "down":
		return c.runClusterDown(ctx, args[1:])
	case "kubeconfig":
		return c.runClusterOutput(ctx, "kubeconfig", args[1:])
	case "talosconfig":
		return c.runClusterOutput(ctx, "talosconfig", args[1:])
	case "doctor":
		return c.runClusterDoctor(ctx, args[1:])
	default:
		fmt.Fprintf(c.stderr, "platformctl cluster: unknown subcommand %q\n", args[0])
		c.printClusterUsage()
		return 2
	}
}

func (c *Command) runTopLevelClusterAction(ctx context.Context, action string, args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(c.stderr, "platformctl %s: environment argument is required\n", action)
		fmt.Fprintf(c.stderr, "usage: platformctl %s <env> --yes [flags]\n", action)
		return 2
	}

	clusterArgs := append([]string{"--env", args[0]}, args[1:]...)
	switch action {
	case "up":
		return c.runClusterUp(ctx, clusterArgs)
	case "down":
		return c.runClusterDown(ctx, clusterArgs)
	default:
		fmt.Fprintf(c.stderr, "platformctl: unknown cluster action %q\n", action)
		return 2
	}
}

func (c *Command) runTopLevelOutput(ctx context.Context, outputName string, args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(c.stderr, "platformctl %s: environment argument is required\n", outputName)
		fmt.Fprintf(c.stderr, "usage: platformctl %s <env> [--out path] [flags]\n", outputName)
		return 2
	}

	clusterArgs := append([]string{"--env", args[0]}, args[1:]...)
	return c.runClusterOutput(ctx, outputName, clusterArgs)
}

func (c *Command) runTopLevelDoctor(ctx context.Context, args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(c.stderr, "platformctl doctor: environment argument is required")
		fmt.Fprintln(c.stderr, "usage: platformctl doctor <env> [flags]")
		return 2
	}

	clusterArgs := append([]string{"--env", args[0]}, args[1:]...)
	return c.runClusterDoctor(ctx, clusterArgs)
}

func (c *Command) runClusterPreview(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("platformctl cluster preview", flag.ContinueOnError)
	fs.SetOutput(c.stderr)

	envName := fs.String("env", DefaultEnvironmentName, "environment name")
	configPath := fs.String("config", DefaultConfigPath, "environment catalog path")
	stackName := fs.String("stack", "", "Pulumi stack name")
	controlPlaneCount := fs.Int("control-plane-count", 0, "override the first control-plane pool count")
	workerCount := fs.Int("worker-count", -1, "override the first worker pool count; use 0 for no workers")
	currentIP := fs.String("current-ip", "", "explicit public IP for current-ip access rules")
	workDir := fs.String("work-dir", "", "Pulumi Automation API work directory")
	backendURL := fs.String("pulumi-backend-url", "", "Pulumi backend URL")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(c.stderr, "platformctl cluster preview: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if *stackName == "" {
		*stackName = *envName
	}
	if *workDir == "" {
		*workDir = filepath.Join(c.workDir, filepath.FromSlash(defaultPulumiWorkspaceDir))
	}

	envVars, err := c.pulumiEnvVars(*backendURL)
	if err != nil {
		fmt.Fprintf(c.stderr, "platformctl cluster preview: %v\n", err)
		return 2
	}

	result, err := c.previewer.PreviewCluster(ctx, deploy.PreviewOptions{
		ProjectName:        DefaultPulumiProjectName,
		StackName:          *stackName,
		WorkDir:            *workDir,
		ConfigPath:         *configPath,
		EnvironmentName:    *envName,
		ControlPlaneCount:  *controlPlaneCount,
		WorkerCount:        *workerCount,
		WorkerCountSet:     *workerCount >= 0,
		CurrentIP:          *currentIP,
		ResolveCurrentIPFn: deploy.ResolvePublicIP,
		EnvVars:            envVars,
	})
	if err != nil {
		fmt.Fprintf(c.stderr, "platformctl cluster preview: %v\n", err)
		return 1
	}

	c.printPreviewResult(*envName, result)
	return 0
}

func (c *Command) runClusterOutput(ctx context.Context, outputName string, args []string) int {
	commandName := "platformctl cluster " + outputName
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	fs.SetOutput(c.stderr)

	envName := fs.String("env", DefaultEnvironmentName, "environment name")
	stackName := fs.String("stack", "", "Pulumi stack name")
	workDir := fs.String("work-dir", "", "Pulumi Automation API work directory")
	backendURL := fs.String("pulumi-backend-url", "", "Pulumi backend URL")
	outPath := fs.String("out", "", "write output to this file instead of stdout")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(c.stderr, "%s: unexpected argument %q\n", commandName, fs.Arg(0))
		return 2
	}
	if *stackName == "" {
		*stackName = *envName
	}
	if *workDir == "" {
		*workDir = c.defaultPulumiWorkDir()
	}

	envVars, err := c.pulumiStateEnvVars(*backendURL)
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: %v\n", commandName, err)
		return 2
	}

	result, err := c.previewer.ReadClusterOutput(ctx, deploy.OutputOptions{
		ProjectName: DefaultPulumiProjectName,
		StackName:   *stackName,
		WorkDir:     *workDir,
		EnvVars:     envVars,
		Name:        outputName,
	})
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: %v\n", commandName, err)
		return 1
	}

	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(result.Value), 0o600); err != nil {
			fmt.Fprintf(c.stderr, "%s: write %s: %v\n", commandName, *outPath, err)
			return 1
		}
		fmt.Fprintf(c.stdout, "Wrote %s for environment %q to %s.\n", outputName, *envName, *outPath)
		return 0
	}

	fmt.Fprint(c.stdout, result.Value)
	if !strings.HasSuffix(result.Value, "\n") {
		fmt.Fprintln(c.stdout)
	}
	return 0
}

func (c *Command) runClusterDoctor(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("platformctl cluster doctor", flag.ContinueOnError)
	fs.SetOutput(c.stderr)

	envName := fs.String("env", DefaultEnvironmentName, "environment name")
	stackName := fs.String("stack", "", "Pulumi stack name")
	workDir := fs.String("work-dir", "", "Pulumi Automation API work directory")
	backendURL := fs.String("pulumi-backend-url", "", "Pulumi backend URL")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(c.stderr, "platformctl cluster doctor: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if *stackName == "" {
		*stackName = *envName
	}
	if *workDir == "" {
		*workDir = c.defaultPulumiWorkDir()
	}

	envVars, err := c.pulumiStateEnvVars(*backendURL)
	if err != nil {
		fmt.Fprintf(c.stderr, "platformctl cluster doctor: %v\n", err)
		return 2
	}

	result, err := c.previewer.DoctorCluster(ctx, deploy.DoctorOptions{
		ProjectName: DefaultPulumiProjectName,
		StackName:   *stackName,
		WorkDir:     *workDir,
		EnvVars:     envVars,
	})
	if err != nil {
		if doctorErr, ok := err.(*deploy.DoctorError); ok {
			c.printDoctorResult(*envName, doctorErr.Result)
		}
		fmt.Fprintf(c.stderr, "platformctl cluster doctor: %v\n", err)
		return 1
	}

	c.printDoctorResult(*envName, result)
	return 0
}

func (c *Command) runClusterUp(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("platformctl cluster up", flag.ContinueOnError)
	fs.SetOutput(c.stderr)

	envName := fs.String("env", DefaultEnvironmentName, "environment name")
	configPath := fs.String("config", DefaultConfigPath, "environment catalog path")
	stackName := fs.String("stack", "", "Pulumi stack name")
	controlPlaneCount := fs.Int("control-plane-count", 0, "override the first control-plane pool count")
	workerCount := fs.Int("worker-count", -1, "override the first worker pool count; use 0 for no workers")
	currentIP := fs.String("current-ip", "", "explicit public IP for current-ip access rules")
	workDir := fs.String("work-dir", "", "Pulumi Automation API work directory")
	backendURL := fs.String("pulumi-backend-url", "", "Pulumi backend URL")
	yes := fs.Bool("yes", false, "confirm creation or update of live resources")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(c.stderr, "platformctl cluster up: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if !*yes {
		fmt.Fprintln(c.stderr, "platformctl cluster up: --yes is required because this creates or updates live resources")
		return 2
	}
	if *stackName == "" {
		*stackName = *envName
	}
	if *workDir == "" {
		*workDir = c.defaultPulumiWorkDir()
	}

	envVars, err := c.pulumiEnvVars(*backendURL)
	if err != nil {
		fmt.Fprintf(c.stderr, "platformctl cluster up: %v\n", err)
		return 2
	}

	result, err := c.previewer.UpCluster(ctx, deploy.UpOptions{
		ProjectName:        DefaultPulumiProjectName,
		StackName:          *stackName,
		WorkDir:            *workDir,
		ConfigPath:         *configPath,
		EnvironmentName:    *envName,
		ControlPlaneCount:  *controlPlaneCount,
		WorkerCount:        *workerCount,
		WorkerCountSet:     *workerCount >= 0,
		CurrentIP:          *currentIP,
		ResolveCurrentIPFn: deploy.ResolvePublicIP,
		EnvVars:            envVars,
		Stdout:             c.stdout,
		Stderr:             c.stderr,
	})
	if err != nil {
		fmt.Fprintf(c.stderr, "platformctl cluster up: %v\n", err)
		return 1
	}

	c.printChangeResult("Apply succeeded", *envName, result.StackName, result.ChangeSummary)
	c.printImageEnsureResult(result)
	return 0
}

func (c *Command) runClusterDown(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("platformctl cluster down", flag.ContinueOnError)
	fs.SetOutput(c.stderr)

	envName := fs.String("env", DefaultEnvironmentName, "environment name")
	stackName := fs.String("stack", "", "Pulumi stack name")
	workDir := fs.String("work-dir", "", "Pulumi Automation API work directory")
	backendURL := fs.String("pulumi-backend-url", "", "Pulumi backend URL")
	yes := fs.Bool("yes", false, "confirm destruction of live resources")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(c.stderr, "platformctl cluster down: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if !*yes {
		fmt.Fprintln(c.stderr, "platformctl cluster down: --yes is required because this destroys live resources")
		return 2
	}
	if *stackName == "" {
		*stackName = *envName
	}
	if *workDir == "" {
		*workDir = c.defaultPulumiWorkDir()
	}

	envVars, err := c.pulumiEnvVars(*backendURL)
	if err != nil {
		fmt.Fprintf(c.stderr, "platformctl cluster down: %v\n", err)
		return 2
	}

	result, err := c.previewer.DestroyCluster(ctx, deploy.DestroyOptions{
		ProjectName: DefaultPulumiProjectName,
		StackName:   *stackName,
		WorkDir:     *workDir,
		EnvVars:     envVars,
		Stdout:      c.stdout,
		Stderr:      c.stderr,
	})
	if err != nil {
		fmt.Fprintf(c.stderr, "platformctl cluster down: %v\n", err)
		return 1
	}

	c.printChangeResult("Destroy succeeded", *envName, result.StackName, result.ChangeSummary)
	return 0
}

func (c *Command) pulumiEnvVars(backendURL string) (map[string]string, error) {
	envVars, err := c.pulumiStateEnvVars(backendURL)
	if err != nil {
		return nil, err
	}

	hcloudToken := c.getenv("HCLOUD_TOKEN")
	if hcloudToken == "" {
		hcloudToken = c.getenv("HETZNER_TOKEN")
	}
	if hcloudToken == "" {
		return nil, fmt.Errorf("HCLOUD_TOKEN or HETZNER_TOKEN is required")
	}

	envVars["HCLOUD_TOKEN"] = hcloudToken
	return envVars, nil
}

func (c *Command) pulumiStateEnvVars(backendURL string) (map[string]string, error) {
	passphrase := c.getenv("PULUMI_CONFIG_PASSPHRASE")
	if passphrase == "" {
		return nil, fmt.Errorf("PULUMI_CONFIG_PASSPHRASE is required for passphrase-protected Pulumi state")
	}

	if backendURL == "" {
		backendURL = c.getenv("PULUMI_BACKEND_URL")
	}
	if backendURL == "" {
		backendURL = "file://" + filepath.Join(c.workDir, ".pulumi")
	}

	return map[string]string{
		"PULUMI_BACKEND_URL":       backendURL,
		"PULUMI_CONFIG_PASSPHRASE": passphrase,
	}, nil
}

func (c *Command) defaultPulumiWorkDir() string {
	return filepath.Join(c.workDir, filepath.FromSlash(defaultPulumiWorkspaceDir))
}

func (c *Command) printPreviewResult(envName string, result deploy.PreviewResult) {
	c.printChangeResult("Preview succeeded", envName, result.StackName, result.ChangeSummary)
	fmt.Fprintln(c.stdout, "No resources were created.")
}

func (c *Command) printChangeResult(prefix string, envName string, stackName string, changeSummary map[string]int) {
	fmt.Fprintf(c.stdout, "%s for environment %q (stack %q).\n", prefix, envName, stackName)
	fmt.Fprintln(c.stdout, "Resource changes:")

	printed := false
	for _, op := range []string{"create", "update", "replace", "delete", "same", "refresh", "discard"} {
		count := changeSummary[op]
		if count == 0 {
			continue
		}
		fmt.Fprintf(c.stdout, "  %s: %d\n", op, count)
		printed = true
	}
	if !printed {
		fmt.Fprintln(c.stdout, "  none")
	}
}

func (c *Command) printDoctorResult(envName string, result deploy.DoctorResult) {
	fmt.Fprintf(c.stdout, "Doctor results for environment %q (stack %q).\n", envName, result.StackName)
	for _, check := range result.Checks {
		status := "failed"
		if check.Passed {
			status = "ok"
		}
		if check.Message == "" {
			fmt.Fprintf(c.stdout, "  %s: %s\n", check.Name, status)
			continue
		}
		fmt.Fprintf(c.stdout, "  %s: %s - %s\n", check.Name, status, check.Message)
	}
}

func (c *Command) printImageEnsureResult(result deploy.UpResult) {
	if len(result.ImagesExisting) == 0 && len(result.ImagesCreated) == 0 {
		return
	}

	fmt.Fprintln(c.stdout, "Talos images:")
	for _, image := range result.ImagesExisting {
		fmt.Fprintf(c.stdout, "  reused: %s\n", image)
	}
	for _, image := range result.ImagesCreated {
		fmt.Fprintf(c.stdout, "  created: %s\n", image)
	}
}

func (c *Command) printUsage() {
	fmt.Fprintln(c.stderr, "usage: platformctl <command> [args]")
}

func (c *Command) printClusterUsage() {
	fmt.Fprintln(c.stderr, "usage: platformctl cluster <preview|up|down|doctor|kubeconfig|talosconfig> [flags]")
}
