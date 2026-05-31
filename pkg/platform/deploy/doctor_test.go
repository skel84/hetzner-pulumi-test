package deploy

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDoctorClusterRunsExpectedChecks(t *testing.T) {
	t.Parallel()

	var commands [][]string
	result, err := DoctorCluster(context.Background(), DoctorOptions{
		ProjectName: "hetzner-pulumi-cluster",
		StackName:   "dev",
		WorkDir:     t.TempDir(),
		EnvVars:     map[string]string{"PULUMI_CONFIG_PASSPHRASE": "secret-passphrase"},
		ReadOutputFn: func(context.Context, OutputOptions) (OutputResult, error) {
			return OutputResult{StackName: "dev", Name: "kubeconfig", Value: "apiVersion: v1", Secret: true}, nil
		},
		RunKubeCommand: func(_ context.Context, _ string, args ...string) (string, error) {
			commands = append(commands, append([]string{}, args...))
			return "ok", nil
		},
	})
	if err != nil {
		t.Fatalf("DoctorCluster() error = %v", err)
	}
	if result.StackName != "dev" {
		t.Fatalf("StackName = %q, want dev", result.StackName)
	}
	if got := len(result.Checks); got != 7 {
		t.Fatalf("len(Checks) = %d, want 7", got)
	}
	for _, check := range result.Checks {
		if !check.Passed {
			t.Fatalf("check %#v failed, want all checks passing", check)
		}
	}

	wantSubstrings := []string{
		"wait --for=condition=Ready nodes --all",
		"rollout status daemonset/cilium",
		"rollout status deployment/hcloud-cloud-controller-manager",
		"rollout status statefulset/argocd-application-controller",
		"rollout status deployment/argocd-server",
		"rollout status deployment/pulumi-kubernetes-operator-controller-manager",
	}
	joinedCommands := make([]string, 0, len(commands))
	for _, command := range commands {
		joinedCommands = append(joinedCommands, strings.Join(command, " "))
	}
	for _, want := range wantSubstrings {
		if !containsCommand(joinedCommands, want) {
			t.Fatalf("commands = %#v, want command containing %q", joinedCommands, want)
		}
	}
}

func TestDoctorClusterReturnsFailedCheck(t *testing.T) {
	t.Parallel()

	_, err := DoctorCluster(context.Background(), DoctorOptions{
		ProjectName: "hetzner-pulumi-cluster",
		StackName:   "dev",
		WorkDir:     t.TempDir(),
		ReadOutputFn: func(context.Context, OutputOptions) (OutputResult, error) {
			return OutputResult{StackName: "dev", Name: "kubeconfig", Value: "apiVersion: v1", Secret: true}, nil
		},
		RunKubeCommand: func(_ context.Context, _ string, args ...string) (string, error) {
			if strings.Contains(strings.Join(args, " "), "daemonset/cilium") {
				return "not ready", errDoctorTest
			}
			return "ok", nil
		},
	})
	if err == nil {
		t.Fatal("DoctorCluster() error = nil, want error")
	}
	doctorErr, ok := err.(*DoctorError)
	if !ok {
		t.Fatalf("DoctorCluster() error type = %T, want *DoctorError", err)
	}
	if !strings.Contains(doctorErr.Error(), "Cilium") {
		t.Fatalf("DoctorCluster() error = %q, want Cilium context", doctorErr.Error())
	}
	if got := doctorErr.Result.Checks[len(doctorErr.Result.Checks)-1]; got.Passed || got.Name != "Cilium" {
		t.Fatalf("last check = %#v, want failed Cilium", got)
	}
}

func TestDoctorClusterReturnsOutputError(t *testing.T) {
	t.Parallel()

	_, err := DoctorCluster(context.Background(), DoctorOptions{
		ProjectName: "hetzner-pulumi-cluster",
		StackName:   "dev",
		WorkDir:     t.TempDir(),
		ReadOutputFn: func(context.Context, OutputOptions) (OutputResult, error) {
			return OutputResult{}, errDoctorTest
		},
		RunKubeCommand: func(context.Context, string, ...string) (string, error) {
			t.Fatal("RunKubeCommand should not be called after output error")
			return "", nil
		},
	})
	if err == nil {
		t.Fatal("DoctorCluster() error = nil, want error")
	}
	doctorErr, ok := err.(*DoctorError)
	if !ok {
		t.Fatalf("DoctorCluster() error type = %T, want *DoctorError", err)
	}
	if doctorErr.Check.Name != "Pulumi stack output" {
		t.Fatalf("failed check = %q, want Pulumi stack output", doctorErr.Check.Name)
	}
}

func TestDoctorErrorWithoutInnerError(t *testing.T) {
	t.Parallel()

	err := (&DoctorError{Check: DoctorCheck{Name: "Cilium"}}).Error()
	if !strings.Contains(err, "Cilium") {
		t.Fatalf("DoctorError.Error() = %q, want check name", err)
	}
}

func TestDoctorClusterValidatesRequiredOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options DoctorOptions
		want    string
	}{
		{
			name:    "project",
			options: DoctorOptions{StackName: "dev", WorkDir: t.TempDir()},
			want:    "project name",
		},
		{
			name:    "stack",
			options: DoctorOptions{ProjectName: "project", WorkDir: t.TempDir()},
			want:    "stack name",
		},
		{
			name:    "workdir",
			options: DoctorOptions{ProjectName: "project", StackName: "dev"},
			want:    "Pulumi work directory",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := DoctorCluster(context.Background(), test.options)
			if err == nil {
				t.Fatal("DoctorCluster() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DoctorCluster() error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestWriteTempKubeconfigCreatesPrivateFileAndCleansUp(t *testing.T) {
	t.Parallel()

	path, cleanup, err := writeTempKubeconfig("apiVersion: v1")
	if err != nil {
		t.Fatalf("writeTempKubeconfig() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat temp kubeconfig: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("temp kubeconfig mode = %o, want 600", got)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp kubeconfig: %v", err)
	}
	if string(contents) != "apiVersion: v1" {
		t.Fatalf("temp kubeconfig contents = %q, want written config", contents)
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stat after cleanup error = %v, want not exist", err)
	}
}

var errDoctorTest = &testError{text: "check failed"}

type testError struct {
	text string
}

func (e *testError) Error() string {
	return e.text
}

func containsCommand(commands []string, want string) bool {
	for _, command := range commands {
		if strings.Contains(command, want) {
			return true
		}
	}

	return false
}
