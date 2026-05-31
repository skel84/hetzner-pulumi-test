package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const defaultDoctorTimeout = 120 * time.Second

type DoctorOptions struct {
	ProjectName string
	StackName   string
	WorkDir     string
	EnvVars     map[string]string
	Timeout     time.Duration

	ReadOutputFn   func(context.Context, OutputOptions) (OutputResult, error)
	RunKubeCommand KubeCommandRunner
}

type DoctorResult struct {
	StackName string
	Checks    []DoctorCheck
}

type DoctorCheck struct {
	Name    string
	Passed  bool
	Message string
}

type DoctorError struct {
	Result DoctorResult
	Check  DoctorCheck
	Err    error
}

type KubeCommandRunner func(context.Context, string, ...string) (string, error)

func (e *DoctorError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("doctor check %q failed", e.Check.Name)
	}

	return fmt.Sprintf("doctor check %q failed: %v", e.Check.Name, e.Err)
}

func DoctorCluster(ctx context.Context, opts DoctorOptions) (DoctorResult, error) {
	if opts.ProjectName == "" {
		return DoctorResult{}, fmt.Errorf("project name is required")
	}
	if opts.StackName == "" {
		return DoctorResult{}, fmt.Errorf("stack name is required")
	}
	if opts.WorkDir == "" {
		return DoctorResult{}, fmt.Errorf("Pulumi work directory is required")
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultDoctorTimeout
	}
	reader := opts.ReadOutputFn
	if reader == nil {
		reader = ReadClusterOutput
	}
	runner := opts.RunKubeCommand
	if runner == nil {
		runner = runKubectl
	}

	result := DoctorResult{StackName: opts.StackName}
	kubeconfig, err := reader(ctx, OutputOptions{
		ProjectName: opts.ProjectName,
		StackName:   opts.StackName,
		WorkDir:     opts.WorkDir,
		EnvVars:     copyStringMap(opts.EnvVars),
		Name:        "kubeconfig",
	})
	if err != nil {
		check := DoctorCheck{Name: "Pulumi stack output", Passed: false, Message: err.Error()}
		result.Checks = append(result.Checks, check)
		return result, &DoctorError{Result: result, Check: check, Err: err}
	}
	result.Checks = append(result.Checks, DoctorCheck{Name: "Pulumi stack output", Passed: true, Message: "kubeconfig output is available"})

	kubeconfigPath, cleanup, err := writeTempKubeconfig(kubeconfig.Value)
	if err != nil {
		check := DoctorCheck{Name: "Temporary kubeconfig", Passed: false, Message: err.Error()}
		result.Checks = append(result.Checks, check)
		return result, &DoctorError{Result: result, Check: check, Err: err}
	}
	defer cleanup()

	checks := []struct {
		name string
		args []string
	}{
		{name: "Kubernetes nodes", args: []string{"wait", "--for=condition=Ready", "nodes", "--all", "--timeout=" + timeout.String()}},
		{name: "Cilium", args: []string{"-n", "kube-system", "rollout", "status", "daemonset/cilium", "--timeout=" + timeout.String()}},
		{name: "Hetzner CCM", args: []string{"-n", "kube-system", "rollout", "status", "deployment/hcloud-cloud-controller-manager", "--timeout=" + timeout.String()}},
		{name: "Argo CD application controller", args: []string{"-n", "platform-gitops", "rollout", "status", "statefulset/argocd-application-controller", "--timeout=" + timeout.String()}},
		{name: "Argo CD server", args: []string{"-n", "platform-gitops", "rollout", "status", "deployment/argocd-server", "--timeout=" + timeout.String()}},
		{name: "Pulumi Kubernetes Operator", args: []string{"-n", "platform-pulumi", "rollout", "status", "deployment/pulumi-kubernetes-operator-controller-manager", "--timeout=" + timeout.String()}},
	}
	for _, checkSpec := range checks {
		output, err := runner(ctx, kubeconfigPath, checkSpec.args...)
		if err != nil {
			check := DoctorCheck{Name: checkSpec.name, Passed: false, Message: strings.TrimSpace(output)}
			result.Checks = append(result.Checks, check)
			return result, &DoctorError{Result: result, Check: check, Err: err}
		}
		result.Checks = append(result.Checks, DoctorCheck{Name: checkSpec.name, Passed: true, Message: strings.TrimSpace(output)})
	}

	return result, nil
}

func writeTempKubeconfig(contents string) (string, func(), error) {
	file, err := os.CreateTemp("", "platformctl-kubeconfig-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("create temporary kubeconfig: %w", err)
	}
	cleanup := func() {
		_ = os.Remove(file.Name())
	}

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("secure temporary kubeconfig: %w", err)
	}
	if _, err := file.WriteString(contents); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("write temporary kubeconfig: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close temporary kubeconfig: %w", err)
	}

	return file.Name(), cleanup, nil
}

func runKubectl(ctx context.Context, kubeconfigPath string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "kubectl", args...)
	command.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
	output, err := command.CombinedOutput()

	return string(output), err
}
