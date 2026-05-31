package validation_test

import (
	"strings"
	"testing"

	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
	"github.com/francesco/hetzner_pulumi/pkg/platform/validation"
)

func TestValidateCatalogAcceptsValidConfig(t *testing.T) {
	t.Parallel()

	if err := validation.ValidateCatalog(validCatalog()); err != nil {
		t.Fatalf("ValidateCatalog() error = %v", err)
	}
}

func TestValidateCatalogRejectsInvalidConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*config.EnvironmentCatalog)
		wantErr string
	}{
		{
			name: "empty cluster name",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.Cluster.Name = ""
					return env
				})
			},
			wantErr: "cluster.name",
		},
		{
			name: "uppercase cluster name",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.Cluster.Name = "Dev-EU-1"
					return env
				})
			},
			wantErr: "cluster.name",
		},
		{
			name: "invalid network cidr",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.Network.CIDR = "10.40.0.1/16"
					return env
				})
			},
			wantErr: "network.cidr",
		},
		{
			name: "missing control plane pool",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.NodePools.ControlPlane = nil
					return env
				})
			},
			wantErr: "nodePools.controlPlane",
		},
		{
			name: "even control plane count",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.NodePools.ControlPlane[0].Count = 2
					return env
				})
			},
			wantErr: "nodePools.controlPlane",
		},
		{
			name: "zero worker count",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.NodePools.Workers[0].Count = 0
					return env
				})
			},
			wantErr: "nodePools.workers",
		},
		{
			name: "unknown control plane location",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.NodePools.ControlPlane[0].Location = "moon1"
					return env
				})
			},
			wantErr: "location",
		},
		{
			name: "unknown architecture hint",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.NodePools.ControlPlane[0].Architecture = "riscv64"
					return env
				})
			},
			wantErr: "architecture",
		},
		{
			name: "x86 server type with arm architecture hint",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.NodePools.ControlPlane[0].Type = "cpx31"
					env.NodePools.ControlPlane[0].Architecture = "arm64"
					return env
				})
			},
			wantErr: "architecture",
		},
		{
			name: "arm server type with amd architecture hint",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.NodePools.ControlPlane[0].Type = "cax31"
					env.NodePools.ControlPlane[0].Architecture = "amd64"
					return env
				})
			},
			wantErr: "architecture",
		},
		{
			name: "unknown access mode",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.Access.Mode = "public"
					return env
				})
			},
			wantErr: "access.mode",
		},
		{
			name: "restricted public without allowed cidrs",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.Access.AllowedCIDRs = nil
					return env
				})
			},
			wantErr: "access.allowedCidrs",
		},
		{
			name: "gitops repo with unsupported scheme",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.GitOps.RepoURL = "ftp://example.com/platform.git"
					env.GitOps.RootPath = "gitops/root"
					return env
				})
			},
			wantErr: "gitops.repoUrl",
		},
		{
			name: "gitops absolute root path",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.GitOps.RepoURL = "https://github.com/example/platform.git"
					env.GitOps.RootPath = "/gitops/root"
					return env
				})
			},
			wantErr: "gitops.rootPath",
		},
		{
			name: "gitops root path parent traversal",
			mutate: func(catalog *config.EnvironmentCatalog) {
				updateDev(catalog, func(env config.EnvironmentSpec) config.EnvironmentSpec {
					env.GitOps.RepoURL = "https://github.com/example/platform.git"
					env.GitOps.RootPath = "../root"
					return env
				})
			},
			wantErr: "gitops.rootPath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			catalog := validCatalog()
			tt.mutate(&catalog)

			err := validation.ValidateCatalog(catalog)
			if err == nil {
				t.Fatal("ValidateCatalog() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateCatalog() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func updateDev(catalog *config.EnvironmentCatalog, update func(config.EnvironmentSpec) config.EnvironmentSpec) {
	catalog.Environments["dev"] = update(catalog.Environments["dev"])
}

func validCatalog() config.EnvironmentCatalog {
	return config.EnvironmentCatalog{
		Environments: map[string]config.EnvironmentSpec{
			"dev": {
				Provider: "hcloud",
				Cluster: config.ClusterSpec{
					Name:              "dev-eu-1",
					Region:            "eu-central",
					TalosVersion:      "v1.12.0",
					KubernetesVersion: "v1.34.0",
				},
				Network: config.NetworkSpec{
					CIDR: "10.40.0.0/16",
				},
				Access: config.AccessSpec{
					Mode:         config.AccessModeRestrictedPublic,
					AllowedCIDRs: []string{"current-ip", "203.0.113.0/24"},
				},
				NodePools: config.NodePoolsSpec{
					ControlPlane: []config.NodePoolSpec{
						{
							Name:         "cp",
							Type:         "cpx31",
							Location:     "nbg1",
							Architecture: "amd64",
							Count:        3,
						},
					},
					Workers: []config.NodePoolSpec{
						{
							Name:         "worker",
							Type:         "cpx31",
							Location:     "nbg1",
							Architecture: "amd64",
							Count:        2,
						},
					},
				},
				Packages: config.PackageProfile{
					ClusterBaseline:    true,
					GitOpsControlPlane: true,
					Secrets:            true,
					CertsDNS:           true,
				},
			},
		},
	}
}
