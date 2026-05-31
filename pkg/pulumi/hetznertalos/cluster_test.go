package hetznertalos

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func TestNewClusterRegistersBaseResources(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := NewCluster(ctx, "dev-eu-1", validClusterArgs())
		return err
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	for _, name := range []string{
		"dev-eu-1-hcloud",
		"dev-eu-1-network",
		"dev-eu-1-control-plane-subnet",
		"dev-eu-1-worker-subnet",
		"dev-eu-1-load-balancer-subnet",
		"dev-eu-1-control-plane-placement",
		"dev-eu-1-control-plane-firewall",
		"dev-eu-1-worker-firewall",
	} {
		if !mocks.hasResource(name) {
			t.Fatalf("expected resource %q, got %#v", name, mocks.names())
		}
	}

	network := mocks.resourceByName("dev-eu-1-network")
	if got := network.inputs["ipRange"].StringValue(); got != "10.40.0.0/16" {
		t.Fatalf("network ipRange = %q, want 10.40.0.0/16", got)
	}

	controlPlaneSubnet := mocks.resourceByName("dev-eu-1-control-plane-subnet")
	if got := controlPlaneSubnet.inputs["ipRange"].StringValue(); got != "10.40.0.0/24" {
		t.Fatalf("control-plane subnet ipRange = %q, want 10.40.0.0/24", got)
	}

	workerSubnet := mocks.resourceByName("dev-eu-1-worker-subnet")
	if got := workerSubnet.inputs["ipRange"].StringValue(); got != "10.40.1.0/24" {
		t.Fatalf("worker subnet ipRange = %q, want 10.40.1.0/24", got)
	}
}

func TestNewClusterRegistersNodeAndLoadBalancerResources(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster, err := NewCluster(ctx, "dev-eu-1", validClusterArgs())
		if err != nil {
			return err
		}

		if got := len(cluster.ControlPlaneNodes); got != 3 {
			t.Fatalf("len(ControlPlaneNodes) = %d, want 3", got)
		}
		if got := len(cluster.WorkerNodes); got != 2 {
			t.Fatalf("len(WorkerNodes) = %d, want 2", got)
		}

		return nil
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	for _, name := range []string{
		"dev-eu-1-cp-0",
		"dev-eu-1-cp-1",
		"dev-eu-1-cp-2",
		"dev-eu-1-worker-0",
		"dev-eu-1-worker-1",
		"dev-eu-1-cp-0-network",
		"dev-eu-1-cp-1-network",
		"dev-eu-1-cp-2-network",
		"dev-eu-1-worker-0-network",
		"dev-eu-1-worker-1-network",
		"dev-eu-1-kube-api-lb",
		"dev-eu-1-kube-api-lb-network",
		"dev-eu-1-kube-api-service",
		"dev-eu-1-kube-api-target-0",
		"dev-eu-1-kube-api-target-1",
		"dev-eu-1-kube-api-target-2",
	} {
		if !mocks.hasResource(name) {
			t.Fatalf("expected resource %q, got %#v", name, mocks.names())
		}
	}

	cp0 := mocks.resourceByName("dev-eu-1-cp-0")
	if got := cp0.inputs["image"].StringValue(); got != "talos-x86-v1.12.0" {
		t.Fatalf("cp0 image = %q, want talos-x86-v1.12.0", got)
	}

	cp0Network := mocks.resourceByName("dev-eu-1-cp-0-network")
	if got := cp0Network.inputs["ip"].StringValue(); got != "10.40.0.10" {
		t.Fatalf("cp0 private IP = %q, want 10.40.0.10", got)
	}

	worker0Network := mocks.resourceByName("dev-eu-1-worker-0-network")
	if got := worker0Network.inputs["ip"].StringValue(); got != "10.40.1.10" {
		t.Fatalf("worker0 private IP = %q, want 10.40.1.10", got)
	}

	lbNetwork := mocks.resourceByName("dev-eu-1-kube-api-lb-network")
	if got := lbNetwork.inputs["ip"].StringValue(); got != "10.40.2.10" {
		t.Fatalf("load balancer private IP = %q, want 10.40.2.10", got)
	}

	service := mocks.resourceByName("dev-eu-1-kube-api-service")
	if got := service.inputs["listenPort"].NumberValue(); got != 6443 {
		t.Fatalf("kube API listenPort = %v, want 6443", got)
	}
	if got := service.inputs["destinationPort"].NumberValue(); got != 6443 {
		t.Fatalf("kube API destinationPort = %v, want 6443", got)
	}

	target := mocks.resourceByName("dev-eu-1-kube-api-target-0")
	if got := target.inputs["usePrivateIp"].BoolValue(); !got {
		t.Fatal("kube API target usePrivateIp = false, want true")
	}
}

func TestNewClusterRejectsUnresolvedCurrentIP(t *testing.T) {
	t.Parallel()

	args := validClusterArgs()
	args.Access.AllowedCIDRs = []string{"current-ip"}

	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := NewCluster(ctx, "dev-eu-1", args)
		return err
	}, pulumi.WithMocks("project", "stack", &recordingMocks{}))

	if err == nil {
		t.Fatal("pulumi.RunErr() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "current-ip") {
		t.Fatalf("pulumi.RunErr() error = %q, want current-ip context", err)
	}
}

func validClusterArgs() ClusterArgs {
	return ClusterArgs{
		ClusterName:       "dev-eu-1",
		Region:            "eu-central",
		TalosVersion:      "v1.12.0",
		KubernetesVersion: "v1.34.0",
		NetworkCIDR:       "10.40.0.0/16",
		Access: AccessArgs{
			Mode:         "restricted-public",
			AllowedCIDRs: []string{"203.0.113.10/32"},
		},
		ControlPlanePools: []NodePoolSpec{
			{
				Name:         "cp",
				Type:         "cpx31",
				Location:     "nbg1",
				Architecture: "amd64",
				Count:        3,
			},
		},
		WorkerPools: []NodePoolSpec{
			{
				Name:         "worker",
				Type:         "cpx31",
				Location:     "nbg1",
				Architecture: "amd64",
				Count:        2,
			},
		},
	}
}

type recordedResource struct {
	typ    string
	name   string
	inputs resource.PropertyMap
}

type recordedCall struct {
	token string
	args  resource.PropertyMap
}

type recordingMocks struct {
	mu        sync.Mutex
	resources []recordedResource
	calls     []recordedCall
}

func (m *recordingMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resources = append(m.resources, recordedResource{
		typ:    args.TypeToken,
		name:   args.Name,
		inputs: args.Inputs,
	})

	return "123", args.Inputs, nil
}

func (m *recordingMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, recordedCall{
		token: args.Token,
		args:  args.Args,
	})

	return args.Args, nil
}

func (m *recordingMocks) hasResource(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, resource := range m.resources {
		if resource.name == name {
			return true
		}
	}

	return false
}

func (m *recordingMocks) resourceByName(name string) recordedResource {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, resource := range m.resources {
		if resource.name == name {
			return resource
		}
	}

	return recordedResource{}
}

func (m *recordingMocks) names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	names := make([]string, 0, len(m.resources))
	for _, resource := range m.resources {
		names = append(names, fmt.Sprintf("%s (%s)", resource.name, resource.typ))
	}

	return names
}

func (m *recordingMocks) callByMachineType(machineType string) recordedCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.calls {
		if value, ok := call.args["machineType"]; ok && value.StringValue() == machineType {
			return call
		}
	}

	return recordedCall{}
}

func (m *recordingMocks) callByToken(token string) recordedCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.calls {
		if call.token == token {
			return call
		}
	}

	return recordedCall{}
}

func (m *recordingMocks) callTokens() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	tokens := make([]string, 0, len(m.calls))
	for _, call := range m.calls {
		tokens = append(tokens, call.token)
	}

	return tokens
}
