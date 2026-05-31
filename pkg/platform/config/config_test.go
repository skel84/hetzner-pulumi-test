package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileLoadsEnvironmentCatalog(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "environments.yaml")
	contents := []byte(`
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
        - 203.0.113.0/24
    nodePools:
      controlPlane:
        - name: cp
          type: cpx31
          location: nbg1
          architecture: arm64
          count: 3
      workers:
        - name: worker
          type: cpx31
          location: nbg1
          architecture: arm64
          count: 2
    packages:
      clusterBaseline: true
      gitopsControlPlane: true
      secrets: true
      certsDns: true
    gitops:
      repoUrl: https://github.com/example/platform.git
      targetRevision: main
      rootPath: gitops/root
`)

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	catalog, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	env, ok := catalog.Environments["dev"]
	if !ok {
		t.Fatalf("expected dev environment, got %#v", catalog.Environments)
	}

	if env.Provider != "hcloud" {
		t.Fatalf("Provider = %q, want hcloud", env.Provider)
	}
	if env.Cluster.Name != "dev-eu-1" {
		t.Fatalf("Cluster.Name = %q, want dev-eu-1", env.Cluster.Name)
	}
	if env.Network.CIDR != "10.40.0.0/16" {
		t.Fatalf("Network.CIDR = %q, want 10.40.0.0/16", env.Network.CIDR)
	}
	if env.Access.Mode != AccessModeRestrictedPublic {
		t.Fatalf("Access.Mode = %q, want %q", env.Access.Mode, AccessModeRestrictedPublic)
	}
	if got := len(env.NodePools.ControlPlane); got != 1 {
		t.Fatalf("len(ControlPlane) = %d, want 1", got)
	}
	if got := env.NodePools.ControlPlane[0].Count; got != 3 {
		t.Fatalf("ControlPlane[0].Count = %d, want 3", got)
	}
	if !env.Packages.ClusterBaseline || !env.Packages.GitOpsControlPlane {
		t.Fatalf("expected clusterBaseline and gitopsControlPlane packages enabled, got %#v", env.Packages)
	}
	if env.GitOps.RepoURL != "https://github.com/example/platform.git" {
		t.Fatalf("GitOps.RepoURL = %q, want repo URL", env.GitOps.RepoURL)
	}
	if env.GitOps.TargetRevision != "main" {
		t.Fatalf("GitOps.TargetRevision = %q, want main", env.GitOps.TargetRevision)
	}
	if env.GitOps.RootPath != "gitops/root" {
		t.Fatalf("GitOps.RootPath = %q, want gitops/root", env.GitOps.RootPath)
	}
}

func TestDefaultPath(t *testing.T) {
	t.Parallel()

	if DefaultPath != "config/environments.yaml" {
		t.Fatalf("DefaultPath = %q, want config/environments.yaml", DefaultPath)
	}
}
