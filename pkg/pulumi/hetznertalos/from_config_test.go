package hetznertalos

import (
	"testing"

	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
)

func TestClusterArgsFromEnvironment(t *testing.T) {
	t.Parallel()

	env := config.EnvironmentSpec{
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
			AllowedCIDRs: []string{"current-ip"},
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
	}

	args := ClusterArgsFromEnvironment(env)

	if args.ClusterName != "dev-eu-1" {
		t.Fatalf("ClusterName = %q, want dev-eu-1", args.ClusterName)
	}
	if args.NetworkCIDR != "10.40.0.0/16" {
		t.Fatalf("NetworkCIDR = %q, want 10.40.0.0/16", args.NetworkCIDR)
	}
	if args.Access.Mode != string(config.AccessModeRestrictedPublic) {
		t.Fatalf("Access.Mode = %q, want %q", args.Access.Mode, config.AccessModeRestrictedPublic)
	}
	if got := len(args.ControlPlanePools); got != 1 {
		t.Fatalf("len(ControlPlanePools) = %d, want 1", got)
	}
	if args.ControlPlanePools[0].Architecture != "amd64" {
		t.Fatalf("ControlPlanePools[0].Architecture = %q, want amd64", args.ControlPlanePools[0].Architecture)
	}
	if !args.Packages.ClusterBaseline || !args.Packages.GitOpsControlPlane {
		t.Fatalf("expected clusterBaseline and gitopsControlPlane packages enabled, got %#v", args.Packages)
	}
}

func TestClusterArgsFromEnvironmentCopiesSlices(t *testing.T) {
	t.Parallel()

	env := config.EnvironmentSpec{
		Access: config.AccessSpec{
			AllowedCIDRs: []string{"current-ip"},
		},
		NodePools: config.NodePoolsSpec{
			ControlPlane: []config.NodePoolSpec{
				{Name: "cp", Count: 3},
			},
			Workers: []config.NodePoolSpec{
				{Name: "worker", Count: 2},
			},
		},
	}

	args := ClusterArgsFromEnvironment(env)
	env.Access.AllowedCIDRs[0] = "203.0.113.0/24"
	env.NodePools.ControlPlane[0].Name = "changed"
	env.NodePools.Workers[0].Name = "changed"

	if args.Access.AllowedCIDRs[0] != "current-ip" {
		t.Fatalf("AllowedCIDRs[0] = %q, want current-ip", args.Access.AllowedCIDRs[0])
	}
	if args.ControlPlanePools[0].Name != "cp" {
		t.Fatalf("ControlPlanePools[0].Name = %q, want cp", args.ControlPlanePools[0].Name)
	}
	if args.WorkerPools[0].Name != "worker" {
		t.Fatalf("WorkerPools[0].Name = %q, want worker", args.WorkerPools[0].Name)
	}
}
