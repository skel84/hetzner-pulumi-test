package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestPreviewClusterPreparesEnvironmentAndRunsInlinePreview(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())
	envVars := map[string]string{"HCLOUD_TOKEN": "secret-token"}
	var got automationProgramOptions

	result, err := PreviewCluster(context.Background(), PreviewOptions{
		ProjectName:       "hetzner-pulumi-cluster",
		StackName:         "dev",
		WorkDir:           t.TempDir(),
		ConfigPath:        configPath,
		EnvironmentName:   "dev",
		ControlPlaneCount: 1,
		CurrentIP:         "203.0.113.10",
		EnvVars:           envVars,
		runPreviewFn: func(_ context.Context, opts automationProgramOptions) (PreviewResult, error) {
			got = opts
			opts.EnvVars["HCLOUD_TOKEN"] = "mutated"
			return PreviewResult{
				StackName:     opts.StackName,
				ChangeSummary: map[string]int{"create": 15},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("PreviewCluster() error = %v", err)
	}

	if result.ChangeSummary["create"] != 15 {
		t.Fatalf("create summary = %d, want 15", result.ChangeSummary["create"])
	}
	if got.ProjectName != "hetzner-pulumi-cluster" {
		t.Fatalf("ProjectName = %q, want hetzner-pulumi-cluster", got.ProjectName)
	}
	if got.Environment.NodePools.ControlPlane[0].Count != 1 {
		t.Fatalf("control plane count = %d, want 1", got.Environment.NodePools.ControlPlane[0].Count)
	}
	if got.Environment.Access.AllowedCIDRs[0] != "203.0.113.10/32" {
		t.Fatalf("allowed CIDR = %q, want resolved current IP", got.Environment.Access.AllowedCIDRs[0])
	}
	if envVars["HCLOUD_TOKEN"] != "secret-token" {
		t.Fatal("PreviewCluster mutated caller env vars")
	}
}

func TestPreviewClusterValidatesRequiredOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options PreviewOptions
		want    string
	}{
		{
			name:    "project",
			options: PreviewOptions{StackName: "dev", WorkDir: t.TempDir()},
			want:    "project name is required",
		},
		{
			name:    "stack",
			options: PreviewOptions{ProjectName: "project", WorkDir: t.TempDir()},
			want:    "stack name is required",
		},
		{
			name:    "workdir",
			options: PreviewOptions{ProjectName: "project", StackName: "dev"},
			want:    "Pulumi work directory is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := PreviewCluster(context.Background(), test.options)
			if err == nil {
				t.Fatal("PreviewCluster() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("PreviewCluster() error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestPreviewClusterReturnsRunnerError(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	_, err := PreviewCluster(context.Background(), PreviewOptions{
		ProjectName:       "project",
		StackName:         "dev",
		WorkDir:           t.TempDir(),
		ConfigPath:        configPath,
		EnvironmentName:   "dev",
		ControlPlaneCount: 1,
		CurrentIP:         "203.0.113.10",
		runPreviewFn: func(context.Context, automationProgramOptions) (PreviewResult, error) {
			return PreviewResult{}, errors.New("runner failed")
		},
	})
	if err == nil {
		t.Fatal("PreviewCluster() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "runner failed") {
		t.Fatalf("PreviewCluster() error = %q, want runner error", err)
	}
}

func TestStringifyChangeSummary(t *testing.T) {
	t.Parallel()

	got := stringifyChangeSummary(map[apitype.OpType]int{
		apitype.OpCreate: 2,
		apitype.OpSame:   3,
	})

	if got["create"] != 2 {
		t.Fatalf("create = %d, want 2", got["create"])
	}
	if got["same"] != 3 {
		t.Fatalf("same = %d, want 3", got["same"])
	}
}

func TestRunAutomationPreviewRejectsFileWorkDir(t *testing.T) {
	t.Parallel()

	workDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(workDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write workdir file: %v", err)
	}

	_, err := runAutomationPreview(context.Background(), automationProgramOptions{
		ProjectName: "project",
		StackName:   "dev",
		WorkDir:     workDir,
		Environment: validProgramEnvironment(),
	})
	if err == nil {
		t.Fatal("runAutomationPreview() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "create Pulumi work directory") {
		t.Fatalf("runAutomationPreview() error = %q, want workdir context", err)
	}
}
