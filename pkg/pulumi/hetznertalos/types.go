package hetznertalos

import (
	"github.com/pulumi/pulumi-hcloud/sdk/go/hcloud"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ClusterArgs struct {
	ClusterName       string
	Region            string
	TalosVersion      string
	KubernetesVersion string
	NetworkCIDR       string
	TalosImages       map[string]string
	Access            AccessArgs
	ControlPlanePools []NodePoolSpec
	WorkerPools       []NodePoolSpec
	Packages          PackageProfile
}

type AccessArgs struct {
	Mode         string
	AllowedCIDRs []string
}

type NodePoolSpec struct {
	Name         string
	Type         string
	Location     string
	Architecture string
	Count        int
}

type PackageProfile struct {
	ClusterBaseline    bool
	GitOpsControlPlane bool
	Secrets            bool
	CertsDNS           bool
}

type Cluster struct {
	pulumi.ResourceState

	Provider                   *hcloud.Provider
	Network                    *hcloud.Network
	ControlPlaneSubnet         *hcloud.NetworkSubnet
	WorkerSubnet               *hcloud.NetworkSubnet
	LoadBalancerSubnet         *hcloud.NetworkSubnet
	ControlPlanePlacementGroup *hcloud.PlacementGroup
	ControlPlaneFirewall       *hcloud.Firewall
	WorkerFirewall             *hcloud.Firewall
	KubeAPILoadBalancer        *hcloud.LoadBalancer
	KubeAPILoadBalancerNetwork *hcloud.LoadBalancerNetwork
	KubeAPILoadBalancerService *hcloud.LoadBalancerService
	KubeAPILoadBalancerTargets []*hcloud.LoadBalancerTarget

	Endpoint              pulumi.StringOutput
	Kubeconfig            pulumi.StringOutput
	Talosconfig           pulumi.StringOutput
	TalosImages           map[string]string
	RequiredArchitectures []string
	ControlPlaneNodes     []Node
	WorkerNodes           []Node
	ClusterProfile        ClusterProfile
}

type Node struct {
	Server            *hcloud.Server
	NetworkAttachment *hcloud.ServerNetwork
	Name              string
	Role              string
	Location          string
	Architecture      string
	PrivateIP         string
	PublicIP          pulumi.StringOutput
}

type ClusterProfile struct {
	ClusterName       string
	Region            string
	Endpoint          string
	TalosVersion      string
	KubernetesVersion string
	Packages          PackageProfile
}
