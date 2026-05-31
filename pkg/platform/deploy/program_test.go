package deploy

import (
	"strings"
	"sync"
	"testing"

	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
	"github.com/francesco/hetzner_pulumi/pkg/pulumi/hetznertalos"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func TestPulumiProgramBuildsCluster(t *testing.T) {
	t.Parallel()

	mocks := &deployMocks{}
	err := pulumi.RunErr(
		PulumiProgram(validProgramEnvironment(), map[string]string{"amd64": "9001"}, "secret-token"),
		pulumi.WithMocks("project", "stack", mocks),
	)
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}
	if len(mocks.serverImages) == 0 {
		t.Fatal("PulumiProgram did not create any server resources")
	}
	for _, image := range mocks.serverImages {
		if image != "9001" {
			t.Fatalf("server image = %q, want ensured image ID 9001", image)
		}
	}
}

func TestPulumiProgramBuildsBootstrapBaselineWhenEnabled(t *testing.T) {
	t.Parallel()

	env := validProgramEnvironment()
	env.Packages.ClusterBaseline = true

	mocks := &deployMocks{}
	err := pulumi.RunErr(
		PulumiProgram(env, map[string]string{"amd64": "9001"}, "secret-token"),
		pulumi.WithMocks("project", "stack", mocks),
	)
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	for _, name := range []string{
		"dev-eu-1-bootstrap-k8s",
		"dev-eu-1-bootstrap-hcloud-secret",
		"dev-eu-1-bootstrap-cilium",
		"dev-eu-1-bootstrap-platform-system",
		"dev-eu-1-bootstrap-platform-gitops",
		"dev-eu-1-bootstrap-platform-pulumi",
		"dev-eu-1-bootstrap-platform-system-deny-ingress",
	} {
		if !mocks.hasResource(name) {
			t.Fatalf("expected resource %q, got %#v", name, mocks.names())
		}
	}
}

func TestPulumiProgramBuildsGitOpsControlPlaneWhenEnabled(t *testing.T) {
	t.Parallel()

	env := validProgramEnvironment()
	env.Packages.ClusterBaseline = true
	env.Packages.GitOpsControlPlane = true

	mocks := &deployMocks{}
	err := pulumi.RunErr(
		PulumiProgram(env, map[string]string{"amd64": "9001"}, "secret-token"),
		pulumi.WithMocks("project", "stack", mocks),
	)
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	for _, name := range []string{
		"dev-eu-1-bootstrap-argocd",
		"dev-eu-1-bootstrap-pulumi-kubernetes-operator",
		"dev-eu-1-bootstrap-pulumi-kubernetes-operator-auth-delegator",
	} {
		if !mocks.hasResource(name) {
			t.Fatalf("expected resource %q, got %#v", name, mocks.names())
		}
	}
}

func TestPulumiProgramBuildsGitOpsRootApplicationWhenConfigured(t *testing.T) {
	t.Parallel()

	env := validProgramEnvironment()
	env.Packages.ClusterBaseline = true
	env.Packages.GitOpsControlPlane = true
	env.GitOps = config.GitOpsSpec{
		RepoURL:        "https://github.com/example/platform.git",
		TargetRevision: "main",
		RootPath:       "gitops/root",
	}

	mocks := &deployMocks{}
	err := pulumi.RunErr(
		PulumiProgram(env, map[string]string{"amd64": "9001"}, "secret-token"),
		pulumi.WithMocks("project", "stack", mocks),
	)
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	if !mocks.hasResource("dev-eu-1-bootstrap-root-application") {
		t.Fatalf("expected root application, got %#v", mocks.names())
	}
}

func TestManifestProfileForClusterAllowsControlPlaneSchedulingWithoutWorkers(t *testing.T) {
	t.Parallel()

	env := validProgramEnvironment()
	env.NodePools.Workers = nil
	args := clusterArgsFromTestEnvironment(env)

	profile := manifestProfileForCluster(args)
	if !profile.AllowSchedulingOnControlPlanes {
		t.Fatal("AllowSchedulingOnControlPlanes = false, want true for zero-worker cluster")
	}
}

func TestManifestProfileForClusterKeepsControlPlanesDedicatedWithWorkers(t *testing.T) {
	t.Parallel()

	args := clusterArgsFromTestEnvironment(validProgramEnvironment())

	profile := manifestProfileForCluster(args)
	if profile.AllowSchedulingOnControlPlanes {
		t.Fatal("AllowSchedulingOnControlPlanes = true, want false when workers exist")
	}
}

func clusterArgsFromTestEnvironment(env config.EnvironmentSpec) hetznertalos.ClusterArgs {
	return hetznertalos.ClusterArgsFromEnvironment(env)
}

func validProgramEnvironment() config.EnvironmentSpec {
	return config.EnvironmentSpec{
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
			AllowedCIDRs: []string{"203.0.113.10/32"},
		},
		NodePools: config.NodePoolsSpec{
			ControlPlane: []config.NodePoolSpec{
				{
					Name:         "cp",
					Type:         "cpx31",
					Location:     "nbg1",
					Architecture: "amd64",
					Count:        1,
				},
			},
			Workers: []config.NodePoolSpec{
				{
					Name:         "worker",
					Type:         "cpx31",
					Location:     "nbg1",
					Architecture: "amd64",
					Count:        1,
				},
			},
		},
	}
}

type deployMocks struct {
	mu           sync.Mutex
	serverImages []string
	resources    []string
}

func (m *deployMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resources = append(m.resources, args.Name)
	if strings.HasSuffix(args.TypeToken, ":Server") {
		m.serverImages = append(m.serverImages, args.Inputs["image"].StringValue())
	}
	if args.Name == "dev-eu-1-kubeconfig" {
		args.Inputs["kubeconfigRaw"] = resource.NewStringProperty("apiVersion: v1")
	}

	return "123", args.Inputs, nil
}

func (*deployMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func (m *deployMocks) hasResource(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, resourceName := range m.resources {
		if resourceName == name {
			return true
		}
	}

	return false
}

func (m *deployMocks) names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string{}, m.resources...)
}
