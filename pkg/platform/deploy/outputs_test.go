package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

func TestReadClusterOutputReturnsStringOutput(t *testing.T) {
	t.Parallel()

	result, err := ReadClusterOutput(context.Background(), OutputOptions{
		ProjectName: "hetzner-pulumi-cluster",
		StackName:   "dev",
		WorkDir:     t.TempDir(),
		EnvVars:     map[string]string{"PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"},
		Name:        "kubeconfig",
		readFn: func(context.Context, automationStackOptions) (auto.OutputMap, error) {
			return auto.OutputMap{
				"kubeconfig": {Value: "apiVersion: v1", Secret: true},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("ReadClusterOutput() error = %v", err)
	}
	if result.Value != "apiVersion: v1" {
		t.Fatalf("Value = %q, want kubeconfig", result.Value)
	}
	if !result.Secret {
		t.Fatal("Secret = false, want true")
	}
}

func TestReadClusterOutputValidatesOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options OutputOptions
		want    string
	}{
		{
			name:    "project",
			options: OutputOptions{StackName: "dev", WorkDir: t.TempDir(), Name: "kubeconfig"},
			want:    "project name",
		},
		{
			name:    "stack",
			options: OutputOptions{ProjectName: "project", WorkDir: t.TempDir(), Name: "kubeconfig"},
			want:    "stack name",
		},
		{
			name:    "workdir",
			options: OutputOptions{ProjectName: "project", StackName: "dev", Name: "kubeconfig"},
			want:    "Pulumi work directory",
		},
		{
			name:    "name",
			options: OutputOptions{ProjectName: "project", StackName: "dev", WorkDir: t.TempDir()},
			want:    "output name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := ReadClusterOutput(context.Background(), test.options)
			if err == nil {
				t.Fatal("ReadClusterOutput() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ReadClusterOutput() error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestReadClusterOutputRejectsMissingAndNonStringOutputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		outputs auto.OutputMap
		want    string
	}{
		{name: "missing", outputs: auto.OutputMap{}, want: "not found"},
		{name: "non string", outputs: auto.OutputMap{"kubeconfig": {Value: 42}}, want: "must be a string"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := ReadClusterOutput(context.Background(), OutputOptions{
				ProjectName: "project",
				StackName:   "dev",
				WorkDir:     t.TempDir(),
				Name:        "kubeconfig",
				readFn: func(context.Context, automationStackOptions) (auto.OutputMap, error) {
					return test.outputs, nil
				},
			})
			if err == nil {
				t.Fatal("ReadClusterOutput() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ReadClusterOutput() error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestReadClusterOutputReturnsReaderError(t *testing.T) {
	t.Parallel()

	_, err := ReadClusterOutput(context.Background(), OutputOptions{
		ProjectName: "project",
		StackName:   "dev",
		WorkDir:     t.TempDir(),
		Name:        "kubeconfig",
		readFn: func(context.Context, automationStackOptions) (auto.OutputMap, error) {
			return nil, errors.New("read failed")
		},
	})
	if err == nil {
		t.Fatal("ReadClusterOutput() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("ReadClusterOutput() error = %q, want reader error", err)
	}
}

func TestReadAutomationOutputsRejectsFileWorkDir(t *testing.T) {
	t.Parallel()

	workDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(workDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write workdir file: %v", err)
	}

	_, err := readAutomationOutputs(context.Background(), automationStackOptions{
		ProjectName: "project",
		StackName:   "dev",
		WorkDir:     workDir,
	})
	if err == nil {
		t.Fatal("readAutomationOutputs() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "create Pulumi work directory") {
		t.Fatalf("readAutomationOutputs() error = %q, want workdir context", err)
	}
}
