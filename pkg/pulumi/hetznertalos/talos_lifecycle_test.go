package hetznertalos

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func TestPulumiverseLifecycleGenerateRegistersSecretsAndConfigurations(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		lifecycle := NewPulumiverseLifecycle()
		generated, err := lifecycle.Generate(ctx, "dev-eu-1", TalosGenerateArgs{
			ClusterName:       "dev-eu-1",
			ClusterEndpoint:   pulumi.String("https://10.40.2.10:6443"),
			TalosVersion:      "v1.12.0",
			KubernetesVersion: "v1.34.0",
			ControlPlaneConfigPatches: []string{
				`{"cluster":{"allowSchedulingOnControlPlanes":false}}`,
			},
			WorkerConfigPatches: []string{
				`{"machine":{"install":{"disk":"/dev/sda"}}}`,
			},
		})
		if err != nil {
			return err
		}
		if generated.Secrets == nil {
			t.Fatal("generated.Secrets = nil")
		}

		return nil
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	if !mocks.hasResource("dev-eu-1-talos-secrets") {
		t.Fatalf("expected Talos secrets resource, got %#v", mocks.names())
	}

	controlPlaneCall := mocks.callByMachineType("controlplane")
	if controlPlaneCall.token == "" {
		t.Fatalf("expected controlplane getConfiguration call, got %#v", mocks.callTokens())
	}
	if got := controlPlaneCall.args["clusterEndpoint"].StringValue(); got != "https://10.40.2.10:6443" {
		t.Fatalf("controlplane clusterEndpoint = %q, want https://10.40.2.10:6443", got)
	}

	workerCall := mocks.callByMachineType("worker")
	if workerCall.token == "" {
		t.Fatalf("expected worker getConfiguration call, got %#v", mocks.callTokens())
	}
	if got := workerCall.args["kubernetesVersion"].StringValue(); got != "v1.34.0" {
		t.Fatalf("worker kubernetesVersion = %q, want v1.34.0", got)
	}
}

func TestPulumiverseLifecycleApplyBootstrapOutputsAndHealth(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		lifecycle := NewPulumiverseLifecycle()
		generated, err := lifecycle.Generate(ctx, "dev-eu-1", TalosGenerateArgs{
			ClusterName:       "dev-eu-1",
			ClusterEndpoint:   pulumi.String("https://10.40.2.10:6443"),
			TalosVersion:      "v1.12.0",
			KubernetesVersion: "v1.34.0",
		})
		if err != nil {
			return err
		}

		args := TalosClusterAccessArgs{
			ClusterName: "dev-eu-1",
			Endpoint:    pulumi.String("10.40.2.10"),
			ControlPlaneNodes: []TalosNode{
				{Name: "dev-eu-1-cp-0", Endpoint: pulumi.String("203.0.113.10"), InternalIP: pulumi.String("10.40.0.10")},
				{Name: "dev-eu-1-cp-1", Endpoint: pulumi.String("203.0.113.11"), InternalIP: pulumi.String("10.40.0.11")},
				{Name: "dev-eu-1-cp-2", Endpoint: pulumi.String("203.0.113.12"), InternalIP: pulumi.String("10.40.0.12")},
			},
			WorkerNodes: []TalosNode{
				{Name: "dev-eu-1-worker-0", Endpoint: pulumi.String("203.0.113.20"), InternalIP: pulumi.String("10.40.1.10")},
				{Name: "dev-eu-1-worker-1", Endpoint: pulumi.String("203.0.113.21"), InternalIP: pulumi.String("10.40.1.11")},
			},
		}

		applied, err := lifecycle.Apply(ctx, "dev-eu-1", generated, args)
		if err != nil {
			return err
		}
		if len(applied.RemainingControlPlaneApplies) != 2 {
			t.Fatalf("len(RemainingControlPlaneApplies) = %d, want 2", len(applied.RemainingControlPlaneApplies))
		}
		if len(applied.WorkerApplies) != 2 {
			t.Fatalf("len(WorkerApplies) = %d, want 2", len(applied.WorkerApplies))
		}

		if _, err := lifecycle.Kubeconfig(ctx, "dev-eu-1", generated, args, pulumi.DependsOn(applied.Resources())); err != nil {
			return err
		}
		lifecycle.Talosconfig(ctx, generated, args)
		lifecycle.CheckHealth(ctx, generated, args)

		return nil
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	for _, name := range []string{
		"dev-eu-1-talos-apply-control-plane-0",
		"dev-eu-1-talos-bootstrap",
		"dev-eu-1-talos-apply-control-plane-1",
		"dev-eu-1-talos-apply-control-plane-2",
		"dev-eu-1-talos-apply-worker-0",
		"dev-eu-1-talos-apply-worker-1",
		"dev-eu-1-kubeconfig",
	} {
		if !mocks.hasResource(name) {
			t.Fatalf("expected resource %q, got %#v", name, mocks.names())
		}
	}

	initialApply := mocks.resourceByName("dev-eu-1-talos-apply-control-plane-0")
	if got := initialApply.inputs["node"].StringValue(); got != "203.0.113.10" {
		t.Fatalf("initial control plane node = %q, want public endpoint", got)
	}

	bootstrap := mocks.resourceByName("dev-eu-1-talos-bootstrap")
	if got := bootstrap.inputs["node"].StringValue(); got != "203.0.113.10" {
		t.Fatalf("bootstrap node = %q, want public endpoint", got)
	}

	if call := mocks.callByToken("talos:client/getConfiguration:getConfiguration"); call.token == "" {
		t.Fatalf("expected talosconfig call, got %#v", mocks.callTokens())
	}
	if call := mocks.callByToken("talos:cluster/getHealth:getHealth"); call.token == "" {
		t.Fatalf("expected health call, got %#v", mocks.callTokens())
	} else if !call.args["skipKubernetesChecks"].BoolValue() {
		t.Fatal("health call skipKubernetesChecks = false, want true for MVP health gate")
	} else if got := call.args["controlPlaneNodes"].ArrayValue()[0].StringValue(); got != "10.40.0.10" {
		t.Fatalf("health controlPlaneNodes[0] = %q, want private IP", got)
	}
}

func TestPulumiverseLifecycleApplyRequiresControlPlaneNode(t *testing.T) {
	t.Parallel()

	_, err := NewPulumiverseLifecycle().Apply(nil, "dev-eu-1", &GeneratedTalosCluster{}, TalosClusterAccessArgs{})
	if err == nil {
		t.Fatal("Apply() error = nil, want error")
	}
}
