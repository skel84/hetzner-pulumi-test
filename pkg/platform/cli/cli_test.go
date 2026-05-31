package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/francesco/hetzner_pulumi/pkg/platform/deploy"
)

func TestClusterPreviewDispatchesInlineAutomation(t *testing.T) {
	t.Parallel()

	previewer := &recordingPreviewer{
		result: deploy.PreviewResult{
			StackName:     "dev",
			ChangeSummary: map[string]int{"create": 15},
		},
	}
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	workDir := t.TempDir()

	cmd := New(CommandOptions{
		Stdout:    stdout,
		Stderr:    stderr,
		Getenv:    getenv(map[string]string{"HCLOUD_TOKEN": "secret-token", "PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   workDir,
		Previewer: previewer,
	})

	code := cmd.Run(context.Background(), []string{
		"cluster",
		"preview",
		"--env", "dev",
		"--config", "config/environments.yaml",
		"--control-plane-count", "1",
		"--worker-count", "0",
		"--current-ip", "203.0.113.10",
	})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr = %q", code, stderr.String())
	}

	got := previewer.options
	if got.ProjectName != DefaultPulumiProjectName {
		t.Fatalf("ProjectName = %q, want %q", got.ProjectName, DefaultPulumiProjectName)
	}
	if got.StackName != "dev" {
		t.Fatalf("StackName = %q, want dev", got.StackName)
	}
	if got.EnvironmentName != "dev" {
		t.Fatalf("EnvironmentName = %q, want dev", got.EnvironmentName)
	}
	if got.ConfigPath != "config/environments.yaml" {
		t.Fatalf("ConfigPath = %q, want config/environments.yaml", got.ConfigPath)
	}
	if got.ControlPlaneCount != 1 {
		t.Fatalf("ControlPlaneCount = %d, want 1", got.ControlPlaneCount)
	}
	if !got.WorkerCountSet || got.WorkerCount != 0 {
		t.Fatalf("WorkerCount = %d set=%t, want explicit zero", got.WorkerCount, got.WorkerCountSet)
	}
	if got.CurrentIP != "203.0.113.10" {
		t.Fatalf("CurrentIP = %q, want 203.0.113.10", got.CurrentIP)
	}
	if got.WorkDir != filepath.Join(workDir, ".pulumi", "platformctl", "cluster") {
		t.Fatalf("WorkDir = %q, want default platformctl workspace", got.WorkDir)
	}
	if got.EnvVars["HCLOUD_TOKEN"] != "secret-token" {
		t.Fatal("HCLOUD_TOKEN was not forwarded to Automation API env")
	}
	if got.EnvVars["HETZNER_TOKEN"] != "" {
		t.Fatal("HETZNER_TOKEN should not be forwarded when HCLOUD_TOKEN is set")
	}
	if got.EnvVars["PULUMI_CONFIG_PASSPHRASE"] != "secret-passphrase" {
		t.Fatal("PULUMI_CONFIG_PASSPHRASE was not forwarded")
	}
	if got.EnvVars["PULUMI_BACKEND_URL"] != "file://"+filepath.Join(workDir, ".pulumi") {
		t.Fatalf("PULUMI_BACKEND_URL = %q, want local backend", got.EnvVars["PULUMI_BACKEND_URL"])
	}
	if got.ResolveCurrentIPFn == nil {
		t.Fatal("ResolveCurrentIPFn = nil, want default resolver")
	}

	if !strings.Contains(stdout.String(), "create: 15") {
		t.Fatalf("stdout = %q, want create summary", stdout.String())
	}
	if strings.Contains(stdout.String(), "secret-token") || strings.Contains(stderr.String(), "secret-token") {
		t.Fatal("command output leaked HCLOUD_TOKEN")
	}
}

func TestClusterPreviewMapsHetznerTokenToHCloudToken(t *testing.T) {
	t.Parallel()

	previewer := &recordingPreviewer{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    &strings.Builder{},
		Getenv:    getenv(map[string]string{"HETZNER_TOKEN": "hetzner-secret", "PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   t.TempDir(),
		Previewer: previewer,
	})

	code := cmd.Run(context.Background(), []string{"cluster", "preview", "--current-ip", "203.0.113.10"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if previewer.options.EnvVars["HCLOUD_TOKEN"] != "hetzner-secret" {
		t.Fatal("HETZNER_TOKEN was not mapped to HCLOUD_TOKEN")
	}
	if _, ok := previewer.options.EnvVars["HETZNER_TOKEN"]; ok {
		t.Fatal("HETZNER_TOKEN should not be forwarded by name")
	}
}

func TestClusterUpRequiresYes(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    stderr,
		Getenv:    getenv(map[string]string{"HCLOUD_TOKEN": "secret-token", "PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   t.TempDir(),
		Previewer: &recordingPreviewer{},
	})

	code := cmd.Run(context.Background(), []string{"cluster", "up", "--env", "dev"})
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--yes is required") {
		t.Fatalf("stderr = %q, want --yes guard", stderr.String())
	}
}

func TestClusterUpDispatchesInlineAutomation(t *testing.T) {
	t.Parallel()

	operator := &recordingPreviewer{
		upResult: deploy.UpResult{
			StackName:     "dev",
			ChangeSummary: map[string]int{"create": 20},
			ImagesCreated: []string{"talos-x86-v1.12.0"},
		},
	}
	stdout := &strings.Builder{}
	workDir := t.TempDir()

	cmd := New(CommandOptions{
		Stdout:    stdout,
		Stderr:    &strings.Builder{},
		Getenv:    getenv(map[string]string{"HCLOUD_TOKEN": "secret-token", "PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   workDir,
		Previewer: operator,
	})

	code := cmd.Run(context.Background(), []string{
		"cluster",
		"up",
		"--env", "dev",
		"--config", "config/environments.yaml",
		"--control-plane-count", "1",
		"--worker-count", "0",
		"--current-ip", "203.0.113.10",
		"--yes",
	})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}

	got := operator.upOptions
	if got.StackName != "dev" {
		t.Fatalf("StackName = %q, want dev", got.StackName)
	}
	if got.ControlPlaneCount != 1 {
		t.Fatalf("ControlPlaneCount = %d, want 1", got.ControlPlaneCount)
	}
	if !got.WorkerCountSet || got.WorkerCount != 0 {
		t.Fatalf("WorkerCount = %d set=%t, want explicit zero", got.WorkerCount, got.WorkerCountSet)
	}
	if got.WorkDir != filepath.Join(workDir, ".pulumi", "platformctl", "cluster") {
		t.Fatalf("WorkDir = %q, want default platformctl workspace", got.WorkDir)
	}
	if !strings.Contains(stdout.String(), "create: 20") {
		t.Fatalf("stdout = %q, want create summary", stdout.String())
	}
	if !strings.Contains(stdout.String(), "created: talos-x86-v1.12.0") {
		t.Fatalf("stdout = %q, want image ensure summary", stdout.String())
	}
}

func TestTopLevelUpUsesEnvironmentArgument(t *testing.T) {
	t.Parallel()

	operator := &recordingPreviewer{
		upResult: deploy.UpResult{StackName: "dev"},
	}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    &strings.Builder{},
		Getenv:    getenv(map[string]string{"HCLOUD_TOKEN": "secret-token", "PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   t.TempDir(),
		Previewer: operator,
	})

	code := cmd.Run(context.Background(), []string{"up", "dev", "--yes", "--control-plane-count", "1", "--current-ip", "203.0.113.10"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if operator.upOptions.EnvironmentName != "dev" {
		t.Fatalf("EnvironmentName = %q, want dev", operator.upOptions.EnvironmentName)
	}
}

func TestTopLevelKubeconfigWritesOutputToStdout(t *testing.T) {
	t.Parallel()

	operator := &recordingPreviewer{
		outputResult: deploy.OutputResult{StackName: "dev", Name: "kubeconfig", Value: "apiVersion: v1\n", Secret: true},
	}
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	workDir := t.TempDir()

	cmd := New(CommandOptions{
		Stdout:    stdout,
		Stderr:    stderr,
		Getenv:    getenv(map[string]string{"PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   workDir,
		Previewer: operator,
	})

	code := cmd.Run(context.Background(), []string{"kubeconfig", "dev"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stdout.String() != "apiVersion: v1\n" {
		t.Fatalf("stdout = %q, want kubeconfig", stdout.String())
	}
	if operator.outputOptions.Name != "kubeconfig" {
		t.Fatalf("output name = %q, want kubeconfig", operator.outputOptions.Name)
	}
	if operator.outputOptions.WorkDir != filepath.Join(workDir, ".pulumi", "platformctl", "cluster") {
		t.Fatalf("WorkDir = %q, want default platformctl workspace", operator.outputOptions.WorkDir)
	}
	if operator.outputOptions.EnvVars["HCLOUD_TOKEN"] != "" {
		t.Fatal("kubeconfig retrieval should not require or forward HCLOUD_TOKEN")
	}
}

func TestTopLevelTalosconfigWritesOutputToFile(t *testing.T) {
	t.Parallel()

	operator := &recordingPreviewer{
		outputResult: deploy.OutputResult{StackName: "dev", Name: "talosconfig", Value: "context: dev\n", Secret: true},
	}
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	outputPath := filepath.Join(t.TempDir(), "talosconfig")

	cmd := New(CommandOptions{
		Stdout:    stdout,
		Stderr:    stderr,
		Getenv:    getenv(map[string]string{"PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   t.TempDir(),
		Previewer: operator,
	})

	code := cmd.Run(context.Background(), []string{"talosconfig", "dev", "--out", outputPath})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", outputPath, err)
	}
	if string(contents) != "context: dev\n" {
		t.Fatalf("file contents = %q, want talosconfig", contents)
	}
	if strings.Contains(stdout.String(), "context: dev") {
		t.Fatal("talosconfig file write leaked config to stdout")
	}
	if operator.outputOptions.Name != "talosconfig" {
		t.Fatalf("output name = %q, want talosconfig", operator.outputOptions.Name)
	}
}

func TestTopLevelDoctorDispatchesChecks(t *testing.T) {
	t.Parallel()

	operator := &recordingPreviewer{
		doctorResult: deploy.DoctorResult{
			StackName: "dev",
			Checks: []deploy.DoctorCheck{
				{Name: "Kubernetes nodes", Passed: true, Message: "ready"},
				{Name: "Cilium", Passed: true, Message: "rolled out"},
			},
		},
	}
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	cmd := New(CommandOptions{
		Stdout:    stdout,
		Stderr:    stderr,
		Getenv:    getenv(map[string]string{"PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   t.TempDir(),
		Previewer: operator,
	})

	code := cmd.Run(context.Background(), []string{"doctor", "dev"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if operator.doctorOptions.StackName != "dev" {
		t.Fatalf("StackName = %q, want dev", operator.doctorOptions.StackName)
	}
	if !strings.Contains(stdout.String(), "Kubernetes nodes: ok") || !strings.Contains(stdout.String(), "Cilium: ok") {
		t.Fatalf("stdout = %q, want doctor checks", stdout.String())
	}
}

func TestTopLevelActionRequiresEnvironmentArgument(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    stderr,
		Getenv:    getenv(nil),
		WorkDir:   t.TempDir(),
		Previewer: &recordingPreviewer{},
	})

	code := cmd.Run(context.Background(), []string{"up", "--yes"})
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "environment argument is required") {
		t.Fatalf("stderr = %q, want environment argument context", stderr.String())
	}
}

func TestTopLevelOutputRequiresEnvironmentArgument(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    stderr,
		Getenv:    getenv(nil),
		WorkDir:   t.TempDir(),
		Previewer: &recordingPreviewer{},
	})

	code := cmd.Run(context.Background(), []string{"kubeconfig", "--out", "config"})
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "environment argument is required") {
		t.Fatalf("stderr = %q, want environment argument context", stderr.String())
	}
}

func TestTopLevelDoctorRequiresEnvironmentArgument(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    stderr,
		Getenv:    getenv(nil),
		WorkDir:   t.TempDir(),
		Previewer: &recordingPreviewer{},
	})

	code := cmd.Run(context.Background(), []string{"doctor", "--stack", "dev"})
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "environment argument is required") {
		t.Fatalf("stderr = %q, want environment argument context", stderr.String())
	}
}

func TestClusterDoctorPrintsFailedChecks(t *testing.T) {
	t.Parallel()

	result := deploy.DoctorResult{
		StackName: "dev",
		Checks: []deploy.DoctorCheck{
			{Name: "Kubernetes nodes", Passed: true},
			{Name: "Cilium", Passed: false, Message: "not ready"},
		},
	}
	operator := &recordingPreviewer{
		err: &deploy.DoctorError{
			Result: result,
			Check:  result.Checks[1],
			Err:    errors.New("rollout failed"),
		},
	}
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	cmd := New(CommandOptions{
		Stdout:    stdout,
		Stderr:    stderr,
		Getenv:    getenv(map[string]string{"PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   t.TempDir(),
		Previewer: operator,
	})

	code := cmd.Run(context.Background(), []string{"cluster", "doctor", "--env", "dev"})
	if code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "Cilium: failed - not ready") {
		t.Fatalf("stdout = %q, want failed check", stdout.String())
	}
	if !strings.Contains(stderr.String(), "doctor check") {
		t.Fatalf("stderr = %q, want doctor error", stderr.String())
	}
}

func TestClusterDownRequiresYes(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    stderr,
		Getenv:    getenv(map[string]string{"HCLOUD_TOKEN": "secret-token", "PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   t.TempDir(),
		Previewer: &recordingPreviewer{},
	})

	code := cmd.Run(context.Background(), []string{"cluster", "down", "--env", "dev"})
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--yes is required") {
		t.Fatalf("stderr = %q, want --yes guard", stderr.String())
	}
}

func TestClusterDownDispatchesInlineAutomation(t *testing.T) {
	t.Parallel()

	operator := &recordingPreviewer{
		destroyResult: deploy.DestroyResult{
			StackName:     "dev",
			ChangeSummary: map[string]int{"delete": 20},
		},
	}
	stdout := &strings.Builder{}
	workDir := t.TempDir()

	cmd := New(CommandOptions{
		Stdout:    stdout,
		Stderr:    &strings.Builder{},
		Getenv:    getenv(map[string]string{"HETZNER_TOKEN": "hetzner-secret", "PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"}),
		WorkDir:   workDir,
		Previewer: operator,
	})

	code := cmd.Run(context.Background(), []string{"down", "dev", "--yes"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}

	got := operator.destroyOptions
	if got.StackName != "dev" {
		t.Fatalf("StackName = %q, want dev", got.StackName)
	}
	if got.EnvVars["HCLOUD_TOKEN"] != "hetzner-secret" {
		t.Fatal("HETZNER_TOKEN was not mapped to HCLOUD_TOKEN")
	}
	if got.WorkDir != filepath.Join(workDir, ".pulumi", "platformctl", "cluster") {
		t.Fatalf("WorkDir = %q, want default platformctl workspace", got.WorkDir)
	}
	if !strings.Contains(stdout.String(), "delete: 20") {
		t.Fatalf("stdout = %q, want delete summary", stdout.String())
	}
}

func TestClusterPreviewRequiresPulumiPassphrase(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    stderr,
		Getenv:    getenv(map[string]string{"HCLOUD_TOKEN": "secret-token"}),
		WorkDir:   t.TempDir(),
		Previewer: &recordingPreviewer{},
	})

	code := cmd.Run(context.Background(), []string{"cluster", "preview"})
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "PULUMI_CONFIG_PASSPHRASE") {
		t.Fatalf("stderr = %q, want passphrase guidance", stderr.String())
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    stderr,
		Getenv:    getenv(nil),
		WorkDir:   t.TempDir(),
		Previewer: &recordingPreviewer{},
	})

	code := cmd.Run(context.Background(), []string{"bogus"})
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown command context", stderr.String())
	}
}

func TestRunRejectsMissingClusterSubcommand(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    stderr,
		Getenv:    getenv(nil),
		WorkDir:   t.TempDir(),
		Previewer: &recordingPreviewer{},
	})

	code := cmd.Run(context.Background(), []string{"cluster"})
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "subcommand is required") {
		t.Fatalf("stderr = %q, want missing subcommand context", stderr.String())
	}
}

func TestRunRejectsUnknownClusterSubcommand(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout:    &strings.Builder{},
		Stderr:    stderr,
		Getenv:    getenv(nil),
		WorkDir:   t.TempDir(),
		Previewer: &recordingPreviewer{},
	})

	code := cmd.Run(context.Background(), []string{"cluster", "bogus"})
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q, want unknown subcommand context", stderr.String())
	}
}

func TestClusterPreviewReturnsPreviewErrors(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	cmd := New(CommandOptions{
		Stdout: &strings.Builder{},
		Stderr: stderr,
		Getenv: getenv(map[string]string{
			"HCLOUD_TOKEN":             "secret-token",
			"PULUMI_CONFIG_PASSPHRASE": "secret-passphrase",
			"PULUMI_BACKEND_URL":       "file:///tmp/pulumi",
		}),
		WorkDir: t.TempDir(),
		Previewer: &recordingPreviewer{
			err: errors.New("preview failed"),
		},
	})

	code := cmd.Run(context.Background(), []string{"cluster", "preview", "--current-ip", "203.0.113.10"})
	if code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "preview failed") {
		t.Fatalf("stderr = %q, want preview error", stderr.String())
	}
}

func TestPreviewerFunc(t *testing.T) {
	t.Parallel()

	called := false
	previewer := PreviewerFunc(func(_ context.Context, opts deploy.PreviewOptions) (deploy.PreviewResult, error) {
		called = true
		if opts.StackName != "dev" {
			t.Fatalf("StackName = %q, want dev", opts.StackName)
		}
		return deploy.PreviewResult{StackName: "dev"}, nil
	})

	result, err := previewer.PreviewCluster(context.Background(), deploy.PreviewOptions{StackName: "dev"})
	if err != nil {
		t.Fatalf("PreviewCluster() error = %v", err)
	}
	if result.StackName != "dev" {
		t.Fatalf("StackName = %q, want dev", result.StackName)
	}
	if !called {
		t.Fatal("PreviewerFunc did not call wrapped function")
	}
}

func TestPreviewerFuncRejectsMutationOperations(t *testing.T) {
	t.Parallel()

	previewer := PreviewerFunc(func(context.Context, deploy.PreviewOptions) (deploy.PreviewResult, error) {
		return deploy.PreviewResult{}, nil
	})

	if _, err := previewer.UpCluster(context.Background(), deploy.UpOptions{}); err == nil {
		t.Fatal("UpCluster() error = nil, want error")
	}
	if _, err := previewer.DestroyCluster(context.Background(), deploy.DestroyOptions{}); err == nil {
		t.Fatal("DestroyCluster() error = nil, want error")
	}
	if _, err := previewer.ReadClusterOutput(context.Background(), deploy.OutputOptions{}); err == nil {
		t.Fatal("ReadClusterOutput() error = nil, want error")
	}
	if _, err := previewer.DoctorCluster(context.Background(), deploy.DoctorOptions{}); err == nil {
		t.Fatal("DoctorCluster() error = nil, want error")
	}
}

func TestDefaultClusterOperatorValidatesOptions(t *testing.T) {
	t.Parallel()

	operator := defaultClusterOperator{}
	if _, err := operator.PreviewCluster(context.Background(), deploy.PreviewOptions{}); err == nil {
		t.Fatal("PreviewCluster() error = nil, want error")
	}
	if _, err := operator.UpCluster(context.Background(), deploy.UpOptions{}); err == nil {
		t.Fatal("UpCluster() error = nil, want error")
	}
	if _, err := operator.DestroyCluster(context.Background(), deploy.DestroyOptions{}); err == nil {
		t.Fatal("DestroyCluster() error = nil, want error")
	}
}

type recordingPreviewer struct {
	options        deploy.PreviewOptions
	upOptions      deploy.UpOptions
	destroyOptions deploy.DestroyOptions
	outputOptions  deploy.OutputOptions
	doctorOptions  deploy.DoctorOptions
	result         deploy.PreviewResult
	upResult       deploy.UpResult
	destroyResult  deploy.DestroyResult
	outputResult   deploy.OutputResult
	doctorResult   deploy.DoctorResult
	err            error
}

func (p *recordingPreviewer) PreviewCluster(_ context.Context, options deploy.PreviewOptions) (deploy.PreviewResult, error) {
	p.options = options
	return p.result, p.err
}

func (p *recordingPreviewer) UpCluster(_ context.Context, options deploy.UpOptions) (deploy.UpResult, error) {
	p.upOptions = options
	return p.upResult, p.err
}

func (p *recordingPreviewer) DestroyCluster(_ context.Context, options deploy.DestroyOptions) (deploy.DestroyResult, error) {
	p.destroyOptions = options
	return p.destroyResult, p.err
}

func (p *recordingPreviewer) ReadClusterOutput(_ context.Context, options deploy.OutputOptions) (deploy.OutputResult, error) {
	p.outputOptions = options
	return p.outputResult, p.err
}

func (p *recordingPreviewer) DoctorCluster(_ context.Context, options deploy.DoctorOptions) (deploy.DoctorResult, error) {
	p.doctorOptions = options
	return p.doctorResult, p.err
}

func getenv(values map[string]string) func(string) string {
	return func(name string) string {
		return values[name]
	}
}
