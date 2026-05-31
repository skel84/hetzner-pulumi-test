package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestUpClusterPreparesEnvironmentAndRunsInlineUp(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())
	envVars := map[string]string{"HCLOUD_TOKEN": "secret-token"}
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	var got automationProgramOptions

	result, err := UpCluster(context.Background(), UpOptions{
		ProjectName:       "hetzner-pulumi-cluster",
		StackName:         "dev",
		WorkDir:           t.TempDir(),
		ConfigPath:        configPath,
		EnvironmentName:   "dev",
		ControlPlaneCount: 1,
		WorkerCount:       0,
		WorkerCountSet:    true,
		CurrentIP:         "203.0.113.10",
		EnvVars:           envVars,
		Stdout:            stdout,
		Stderr:            stderr,
		EnsureImagesFn:    successfulImageEnsurer(t, []string{"talos-x86-v1.12.0"}, map[string]string{"amd64": "9001"}),
		runUpFn: func(_ context.Context, opts automationProgramOptions) (UpResult, error) {
			got = opts
			opts.EnvVars["HCLOUD_TOKEN"] = "mutated"
			return UpResult{
				StackName:     opts.StackName,
				ChangeSummary: map[string]int{"create": 20},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("UpCluster() error = %v", err)
	}

	if result.ChangeSummary["create"] != 20 {
		t.Fatalf("create summary = %d, want 20", result.ChangeSummary["create"])
	}
	if got.Environment.NodePools.ControlPlane[0].Count != 1 {
		t.Fatalf("control plane count = %d, want 1", got.Environment.NodePools.ControlPlane[0].Count)
	}
	if got.Environment.Access.AllowedCIDRs[0] != "203.0.113.10/32" {
		t.Fatalf("allowed CIDR = %q, want resolved current IP", got.Environment.Access.AllowedCIDRs[0])
	}
	if len(got.Environment.NodePools.Workers) != 0 {
		t.Fatalf("workers = %#v, want none", got.Environment.NodePools.Workers)
	}
	if got.ImageRefs["amd64"] != "9001" {
		t.Fatalf("amd64 image ref = %q, want ensured image ID", got.ImageRefs["amd64"])
	}
	if got.HCloudToken != "secret-token" {
		t.Fatal("HCLOUD_TOKEN was not forwarded to the Pulumi program")
	}
	if got.Stdout != stdout {
		t.Fatal("stdout progress writer was not forwarded to automation runner")
	}
	if got.Stderr != stderr {
		t.Fatal("stderr progress writer was not forwarded to automation runner")
	}
	if !reflect.DeepEqual(result.ImagesExisting, []string{"talos-x86-v1.12.0"}) {
		t.Fatalf("ImagesExisting = %#v, want ensured image name", result.ImagesExisting)
	}
	if !strings.Contains(stdout.String(), "Ensuring Talos images") || !strings.Contains(stdout.String(), "Talos image reused: talos-x86-v1.12.0") {
		t.Fatalf("stdout = %q, want image ensure progress", stdout.String())
	}
	if envVars["HCLOUD_TOKEN"] != "secret-token" {
		t.Fatal("UpCluster mutated caller env vars")
	}
}

func TestPrepareAutomationEnvAddsIsolatedHelmPaths(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	envVars := map[string]string{"HCLOUD_TOKEN": "secret-token"}

	got, err := prepareAutomationEnv(workDir, envVars)
	if err != nil {
		t.Fatalf("prepareAutomationEnv() error = %v", err)
	}

	helmDir := filepath.Join(workDir, "helm")
	repositoryConfig := filepath.Join(helmDir, "repositories.yaml")
	repositoryCache := filepath.Join(helmDir, "repository-cache")
	registryConfig := filepath.Join(helmDir, "registry.json")

	if got["PULUMI_K8S_HELM_REPOSITORY_CONFIG_PATH"] != repositoryConfig {
		t.Fatalf("PULUMI_K8S_HELM_REPOSITORY_CONFIG_PATH = %q, want workspace-local repositories.yaml", got["PULUMI_K8S_HELM_REPOSITORY_CONFIG_PATH"])
	}
	if got["PULUMI_K8S_HELM_REPOSITORY_CACHE"] != repositoryCache {
		t.Fatalf("PULUMI_K8S_HELM_REPOSITORY_CACHE = %q, want workspace-local repository cache", got["PULUMI_K8S_HELM_REPOSITORY_CACHE"])
	}
	if got["PULUMI_K8S_HELM_REGISTRY_CONFIG_PATH"] != registryConfig {
		t.Fatalf("PULUMI_K8S_HELM_REGISTRY_CONFIG_PATH = %q, want workspace-local registry config", got["PULUMI_K8S_HELM_REGISTRY_CONFIG_PATH"])
	}
	if got["HELM_REPOSITORY_CONFIG"] != repositoryConfig {
		t.Fatalf("HELM_REPOSITORY_CONFIG = %q, want workspace-local repositories.yaml", got["HELM_REPOSITORY_CONFIG"])
	}
	if got["HELM_REPOSITORY_CACHE"] != repositoryCache {
		t.Fatalf("HELM_REPOSITORY_CACHE = %q, want workspace-local repository cache", got["HELM_REPOSITORY_CACHE"])
	}
	if got["HELM_REGISTRY_CONFIG"] != registryConfig {
		t.Fatalf("HELM_REGISTRY_CONFIG = %q, want workspace-local registry config", got["HELM_REGISTRY_CONFIG"])
	}
	for _, path := range []string{repositoryCache, filepath.Join(helmDir, "config"), filepath.Join(helmDir, "cache"), filepath.Join(helmDir, "data")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("helm workspace path %q stat error = %v", path, err)
		}
	}
	if _, ok := envVars["HELM_REPOSITORY_CONFIG"]; ok {
		t.Fatal("prepareAutomationEnv mutated caller env vars")
	}
}

func TestUpClusterValidatesRequiredOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options UpOptions
		want    string
	}{
		{
			name:    "project",
			options: UpOptions{StackName: "dev", WorkDir: t.TempDir()},
			want:    "project name is required",
		},
		{
			name:    "stack",
			options: UpOptions{ProjectName: "project", WorkDir: t.TempDir()},
			want:    "stack name is required",
		},
		{
			name:    "workdir",
			options: UpOptions{ProjectName: "project", StackName: "dev"},
			want:    "Pulumi work directory is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := UpCluster(context.Background(), test.options)
			if err == nil {
				t.Fatal("UpCluster() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("UpCluster() error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestUpClusterReturnsRunnerError(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	_, err := UpCluster(context.Background(), UpOptions{
		ProjectName:       "project",
		StackName:         "dev",
		WorkDir:           t.TempDir(),
		ConfigPath:        configPath,
		EnvironmentName:   "dev",
		ControlPlaneCount: 1,
		CurrentIP:         "203.0.113.10",
		EnvVars:           map[string]string{"HCLOUD_TOKEN": "secret-token"},
		EnsureImagesFn:    successfulImageEnsurer(t, []string{"talos-x86-v1.12.0"}, map[string]string{"amd64": "9001"}),
		runUpFn: func(context.Context, automationProgramOptions) (UpResult, error) {
			return UpResult{}, errors.New("up failed")
		},
	})
	if err == nil {
		t.Fatal("UpCluster() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "up failed") {
		t.Fatalf("UpCluster() error = %q, want runner error", err)
	}
}

func TestDestroyClusterRunsInlineDestroy(t *testing.T) {
	t.Parallel()

	envVars := map[string]string{"HCLOUD_TOKEN": "secret-token"}
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	var got automationStackOptions

	result, err := DestroyCluster(context.Background(), DestroyOptions{
		ProjectName: "hetzner-pulumi-cluster",
		StackName:   "dev",
		WorkDir:     t.TempDir(),
		EnvVars:     envVars,
		Stdout:      stdout,
		Stderr:      stderr,
		runDestroyFn: func(_ context.Context, opts automationStackOptions) (DestroyResult, error) {
			got = opts
			opts.EnvVars["HCLOUD_TOKEN"] = "mutated"
			return DestroyResult{
				StackName:     opts.StackName,
				ChangeSummary: map[string]int{"delete": 20},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("DestroyCluster() error = %v", err)
	}

	if result.ChangeSummary["delete"] != 20 {
		t.Fatalf("delete summary = %d, want 20", result.ChangeSummary["delete"])
	}
	if got.StackName != "dev" {
		t.Fatalf("StackName = %q, want dev", got.StackName)
	}
	if got.Stdout != stdout {
		t.Fatal("stdout progress writer was not forwarded to destroy runner")
	}
	if got.Stderr != stderr {
		t.Fatal("stderr progress writer was not forwarded to destroy runner")
	}
	if envVars["HCLOUD_TOKEN"] != "secret-token" {
		t.Fatal("DestroyCluster mutated caller env vars")
	}
}

func TestUpClusterReturnsImageCheckErrorBeforeRunner(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())
	runnerCalled := false

	_, err := UpCluster(context.Background(), UpOptions{
		ProjectName:       "project",
		StackName:         "dev",
		WorkDir:           t.TempDir(),
		ConfigPath:        configPath,
		EnvironmentName:   "dev",
		ControlPlaneCount: 1,
		CurrentIP:         "203.0.113.10",
		EnvVars:           map[string]string{"HCLOUD_TOKEN": "secret-token"},
		EnsureImagesFn: func(context.Context, string, []TalosImageSpec) (ImageEnsureResult, error) {
			return ImageEnsureResult{}, errors.New("image upload failed")
		},
		runUpFn: func(context.Context, automationProgramOptions) (UpResult, error) {
			runnerCalled = true
			return UpResult{}, nil
		},
	})
	if err == nil {
		t.Fatal("UpCluster() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "image upload failed") {
		t.Fatalf("UpCluster() error = %q, want image ensure error", err)
	}
	if runnerCalled {
		t.Fatal("UpCluster called runner after image ensure failure")
	}
}

func TestDestroyClusterValidatesRequiredOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options DestroyOptions
		want    string
	}{
		{
			name:    "project",
			options: DestroyOptions{StackName: "dev", WorkDir: t.TempDir()},
			want:    "project name is required",
		},
		{
			name:    "stack",
			options: DestroyOptions{ProjectName: "project", WorkDir: t.TempDir()},
			want:    "stack name is required",
		},
		{
			name:    "workdir",
			options: DestroyOptions{ProjectName: "project", StackName: "dev"},
			want:    "Pulumi work directory is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := DestroyCluster(context.Background(), test.options)
			if err == nil {
				t.Fatal("DestroyCluster() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DestroyCluster() error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestRunAutomationUpRejectsFileWorkDir(t *testing.T) {
	t.Parallel()

	workDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(workDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write workdir file: %v", err)
	}

	_, err := runAutomationUp(context.Background(), automationProgramOptions{
		ProjectName: "project",
		StackName:   "dev",
		WorkDir:     workDir,
		Environment: validProgramEnvironment(),
	})
	if err == nil {
		t.Fatal("runAutomationUp() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "create Pulumi work directory") {
		t.Fatalf("runAutomationUp() error = %q, want workdir context", err)
	}
}

func TestRunAutomationDestroyRejectsFileWorkDir(t *testing.T) {
	t.Parallel()

	workDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(workDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write workdir file: %v", err)
	}

	_, err := runAutomationDestroy(context.Background(), automationStackOptions{
		ProjectName: "project",
		StackName:   "dev",
		WorkDir:     workDir,
	})
	if err == nil {
		t.Fatal("runAutomationDestroy() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "create Pulumi work directory") {
		t.Fatalf("runAutomationDestroy() error = %q, want workdir context", err)
	}
}

func TestChangeSummaryFromUpdate(t *testing.T) {
	t.Parallel()

	if got := changeSummaryFromUpdate(nil); len(got) != 0 {
		t.Fatalf("changeSummaryFromUpdate(nil) = %#v, want empty", got)
	}

	source := map[string]int{"create": 1}
	got := changeSummaryFromUpdate(&source)
	source["create"] = 2
	if got["create"] != 1 {
		t.Fatalf("create = %d, want copied value 1", got["create"])
	}
}

func TestStackOptions(t *testing.T) {
	t.Parallel()

	options := stackOptions("project", t.TempDir(), map[string]string{"HCLOUD_TOKEN": "secret-token"})
	if len(options) != 4 {
		t.Fatalf("len(stackOptions()) = %d, want 4", len(options))
	}
}

func TestAutomationProgressOptions(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	if got := len(upOptions(automationProgramOptions{Stdout: stdout, Stderr: stderr})); got != 3 {
		t.Fatalf("len(upOptions(with streams)) = %d, want 3", got)
	}
	if got := len(upOptions(automationProgramOptions{})); got != 2 {
		t.Fatalf("len(upOptions(without streams)) = %d, want 2", got)
	}
	if got := len(destroyOptions(automationStackOptions{Stdout: stdout, Stderr: stderr})); got != 4 {
		t.Fatalf("len(destroyOptions(with streams)) = %d, want 4", got)
	}
	if got := len(destroyOptions(automationStackOptions{})); got != 3 {
		t.Fatalf("len(destroyOptions(without streams)) = %d, want 3", got)
	}
}

func TestAutomationErrorUsesConciseMessageForStreamedOutput(t *testing.T) {
	t.Parallel()

	err := automationError(errors.New("failed to run update: exit status 1\ncode: 1\nstdout: long pulumi output\nstderr: details"), true)
	if err.Error() != "failed to run update: exit status 1 (exit code 1)" {
		t.Fatalf("automationError(streamed) = %q, want concise message", err)
	}

	verbose := automationError(errors.New("failed\ncode: 1\nstdout: details"), false)
	if !strings.Contains(verbose.Error(), "stdout: details") {
		t.Fatalf("automationError(not streamed) = %q, want original error", verbose)
	}
}

func successfulImageEnsurer(t *testing.T, wantImages []string, refs map[string]string) ImageEnsurer {
	t.Helper()

	return func(_ context.Context, token string, specs []TalosImageSpec) (ImageEnsureResult, error) {
		if token == "" {
			t.Fatal("image ensurer got empty token")
		}
		imageNames := make([]string, 0, len(specs))
		for _, spec := range specs {
			imageNames = append(imageNames, spec.Name)
		}
		if strings.Join(imageNames, ",") != strings.Join(wantImages, ",") {
			t.Fatalf("image names = %#v, want %#v", imageNames, wantImages)
		}
		return ImageEnsureResult{
			Refs:     refs,
			Existing: append([]string{}, wantImages...),
		}, nil
	}
}
