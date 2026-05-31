package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareEnvironmentAppliesControlPlaneOverrideAndCurrentIP(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	prepared, err := PrepareEnvironment(context.Background(), PrepareOptions{
		ConfigPath:         configPath,
		EnvironmentName:    "dev",
		ControlPlaneCount:  1,
		CurrentIP:          "203.0.113.10",
		ResolveCurrentIPFn: unexpectedCurrentIPResolver(t),
	})
	if err != nil {
		t.Fatalf("PrepareEnvironment() error = %v", err)
	}

	if prepared.Name != "dev" {
		t.Fatalf("prepared.Name = %q, want dev", prepared.Name)
	}
	if got := prepared.Environment.NodePools.ControlPlane[0].Count; got != 1 {
		t.Fatalf("control plane count = %d, want 1", got)
	}
	if got := prepared.Environment.Access.AllowedCIDRs; len(got) != 1 || got[0] != "203.0.113.10/32" {
		t.Fatalf("allowed CIDRs = %#v, want current IP /32", got)
	}
	if !prepared.CurrentIPResolved {
		t.Fatal("CurrentIPResolved = false, want true")
	}
}

func TestPrepareEnvironmentUsesCurrentIPResolver(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	prepared, err := PrepareEnvironment(context.Background(), PrepareOptions{
		ConfigPath:      configPath,
		EnvironmentName: "dev",
		ResolveCurrentIPFn: func(context.Context) (string, error) {
			return "2001:db8::1", nil
		},
	})
	if err != nil {
		t.Fatalf("PrepareEnvironment() error = %v", err)
	}

	if got := prepared.Environment.Access.AllowedCIDRs; len(got) != 1 || got[0] != "2001:db8::1/128" {
		t.Fatalf("allowed CIDRs = %#v, want resolver IP /128", got)
	}
}

func TestPrepareEnvironmentAppliesWorkerCountOverride(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	prepared, err := PrepareEnvironment(context.Background(), PrepareOptions{
		ConfigPath:         configPath,
		EnvironmentName:    "dev",
		WorkerCount:        0,
		WorkerCountSet:     true,
		CurrentIP:          "203.0.113.10",
		ResolveCurrentIPFn: unexpectedCurrentIPResolver(t),
	})
	if err != nil {
		t.Fatalf("PrepareEnvironment() error = %v", err)
	}
	if len(prepared.Environment.NodePools.Workers) != 0 {
		t.Fatalf("workers = %#v, want none", prepared.Environment.NodePools.Workers)
	}
}

func TestPrepareEnvironmentAppliesPositiveWorkerCountOverride(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	prepared, err := PrepareEnvironment(context.Background(), PrepareOptions{
		ConfigPath:         configPath,
		EnvironmentName:    "dev",
		WorkerCount:        2,
		WorkerCountSet:     true,
		CurrentIP:          "203.0.113.10",
		ResolveCurrentIPFn: unexpectedCurrentIPResolver(t),
	})
	if err != nil {
		t.Fatalf("PrepareEnvironment() error = %v", err)
	}
	if got := len(prepared.Environment.NodePools.Workers); got != 1 {
		t.Fatalf("len(workers) = %d, want 1", got)
	}
	if got := prepared.Environment.NodePools.Workers[0].Count; got != 2 {
		t.Fatalf("worker count = %d, want 2", got)
	}
}

func TestPrepareEnvironmentRejectsMissingCurrentIPResolver(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	_, err := PrepareEnvironment(context.Background(), PrepareOptions{
		ConfigPath:      configPath,
		EnvironmentName: "dev",
	})
	if err == nil {
		t.Fatal("PrepareEnvironment() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "current-ip") {
		t.Fatalf("PrepareEnvironment() error = %q, want current-ip context", err)
	}
}

func TestPrepareEnvironmentValidatesControlPlaneOverride(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	_, err := PrepareEnvironment(context.Background(), PrepareOptions{
		ConfigPath:         configPath,
		EnvironmentName:    "dev",
		ControlPlaneCount:  2,
		CurrentIP:          "203.0.113.10",
		ResolveCurrentIPFn: unexpectedCurrentIPResolver(t),
	})
	if err == nil {
		t.Fatal("PrepareEnvironment() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "total count must be odd") {
		t.Fatalf("PrepareEnvironment() error = %q, want odd control plane count context", err)
	}
}

func TestPrepareEnvironmentValidatesWorkerOverride(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	_, err := PrepareEnvironment(context.Background(), PrepareOptions{
		ConfigPath:         configPath,
		EnvironmentName:    "dev",
		WorkerCount:        -1,
		WorkerCountSet:     true,
		CurrentIP:          "203.0.113.10",
		ResolveCurrentIPFn: unexpectedCurrentIPResolver(t),
	})
	if err == nil {
		t.Fatal("PrepareEnvironment() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "worker count override") {
		t.Fatalf("PrepareEnvironment() error = %q, want worker override context", err)
	}
}

func TestPrepareEnvironmentRejectsUnknownEnvironment(t *testing.T) {
	t.Parallel()

	configPath := writeEnvironmentConfig(t, validEnvironmentConfig())

	_, err := PrepareEnvironment(context.Background(), PrepareOptions{
		ConfigPath:      configPath,
		EnvironmentName: "prod",
	})
	if err == nil {
		t.Fatal("PrepareEnvironment() error = nil, want error")
	}
	if !strings.Contains(err.Error(), `environment "prod" not found`) {
		t.Fatalf("PrepareEnvironment() error = %q, want missing environment context", err)
	}
}

func writeEnvironmentConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "environments.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	return path
}

func unexpectedCurrentIPResolver(t *testing.T) CurrentIPResolver {
	t.Helper()

	return func(context.Context) (string, error) {
		t.Fatal("current IP resolver was called unexpectedly")
		return "", nil
	}
}

func validEnvironmentConfig() string {
	return `
environments:
  dev:
    provider: hcloud
    cluster:
      name: dev-eu-1
      region: eu-central
      talosVersion: v1.12.0
      kubernetesVersion: v1.34.0
    network:
      cidr: 10.40.0.0/16
    access:
      mode: restricted-public
      allowedCidrs:
        - current-ip
    nodePools:
      controlPlane:
        - name: cp
          type: cpx31
          location: nbg1
          architecture: amd64
          count: 3
      workers:
        - name: worker
          type: cpx31
          location: nbg1
          architecture: amd64
          count: 2
    packages:
      clusterBaseline: true
      gitopsControlPlane: true
      secrets: false
      certsDns: false
`
}
