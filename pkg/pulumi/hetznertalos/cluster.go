package hetznertalos

import (
	"fmt"

	"github.com/pulumi/pulumi-hcloud/sdk/go/hcloud"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const clusterType = "platform:hetznertalos:Cluster"

func NewCluster(ctx *pulumi.Context, name string, args ClusterArgs, opts ...pulumi.ResourceOption) (*Cluster, error) {
	if err := validatePulumiArgs(args); err != nil {
		return nil, err
	}

	cluster := &Cluster{}
	if err := ctx.RegisterComponentResource(clusterType, name, cluster, opts...); err != nil {
		return nil, err
	}

	provider, err := hcloud.NewProvider(ctx, name+"-hcloud", &hcloud.ProviderArgs{}, pulumi.Parent(cluster))
	if err != nil {
		return nil, err
	}
	cluster.Provider = provider

	childOpts := []pulumi.ResourceOption{
		pulumi.Parent(cluster),
		pulumi.Provider(provider),
	}

	subnets, err := DerivedSubnets(args.NetworkCIDR, needsKubeAPILoadBalancer(args))
	if err != nil {
		return nil, err
	}

	cluster.TalosImages = TalosImageReferences(args)
	for architecture, imageRef := range args.TalosImages {
		cluster.TalosImages[architecture] = imageRef
	}
	cluster.RequiredArchitectures = RequiredArchitectures(args)

	network, err := hcloud.NewNetwork(ctx, name+"-network", &hcloud.NetworkArgs{
		Name:    pulumi.String(args.ClusterName),
		IpRange: pulumi.String(args.NetworkCIDR),
		Labels:  labels(args.ClusterName, "network"),
	}, childOpts...)
	if err != nil {
		return nil, err
	}
	cluster.Network = network

	controlPlaneSubnet, err := newSubnet(ctx, name+"-control-plane-subnet", network, args.Region, subnets.ControlPlane, childOpts...)
	if err != nil {
		return nil, err
	}
	cluster.ControlPlaneSubnet = controlPlaneSubnet

	workerSubnet, err := newSubnet(ctx, name+"-worker-subnet", network, args.Region, subnets.Workers, childOpts...)
	if err != nil {
		return nil, err
	}
	cluster.WorkerSubnet = workerSubnet

	if subnets.LoadBalancer != "" {
		loadBalancerSubnet, err := newSubnet(ctx, name+"-load-balancer-subnet", network, args.Region, subnets.LoadBalancer, childOpts...)
		if err != nil {
			return nil, err
		}
		cluster.LoadBalancerSubnet = loadBalancerSubnet
	}

	placementGroup, err := hcloud.NewPlacementGroup(ctx, name+"-control-plane-placement", &hcloud.PlacementGroupArgs{
		Name:   pulumi.String(name + "-control-plane"),
		Type:   pulumi.String("spread"),
		Labels: labels(args.ClusterName, "control-plane"),
	}, childOpts...)
	if err != nil {
		return nil, err
	}
	cluster.ControlPlanePlacementGroup = placementGroup

	controlPlaneFirewall, err := hcloud.NewFirewall(ctx, name+"-control-plane-firewall", &hcloud.FirewallArgs{
		Name:   pulumi.String(name + "-control-plane"),
		Labels: labels(args.ClusterName, "control-plane"),
		Rules:  controlPlaneFirewallRules(args.Access),
	}, childOpts...)
	if err != nil {
		return nil, err
	}
	cluster.ControlPlaneFirewall = controlPlaneFirewall

	workerFirewall, err := hcloud.NewFirewall(ctx, name+"-worker-firewall", &hcloud.FirewallArgs{
		Name:   pulumi.String(name + "-worker"),
		Labels: labels(args.ClusterName, "worker"),
		Rules:  workerFirewallRules(args.Access),
	}, childOpts...)
	if err != nil {
		return nil, err
	}
	cluster.WorkerFirewall = workerFirewall

	controlPlaneNodes, err := createNodePoolResources(ctx, nodePoolBuildArgs{
		ClusterName:    args.ClusterName,
		Role:           "control-plane",
		SubnetCIDR:     subnets.ControlPlane,
		Images:         cluster.TalosImages,
		Network:        network,
		Firewall:       controlPlaneFirewall,
		PlacementGroup: placementGroup,
		Pools:          args.ControlPlanePools,
	}, childOpts...)
	if err != nil {
		return nil, err
	}
	cluster.ControlPlaneNodes = controlPlaneNodes

	workerNodes, err := createNodePoolResources(ctx, nodePoolBuildArgs{
		ClusterName: args.ClusterName,
		Role:        "worker",
		SubnetCIDR:  subnets.Workers,
		Images:      cluster.TalosImages,
		Network:     network,
		Firewall:    workerFirewall,
		Pools:       args.WorkerPools,
	}, childOpts...)
	if err != nil {
		return nil, err
	}
	cluster.WorkerNodes = workerNodes

	if subnets.LoadBalancer != "" {
		loadBalancerIP, err := privateIP(subnets.LoadBalancer, 10)
		if err != nil {
			return nil, err
		}

		loadBalancer, loadBalancerNetwork, service, targets, err := createKubeAPILoadBalancer(ctx, name, args, network, controlPlaneNodes, loadBalancerIP, childOpts...)
		if err != nil {
			return nil, err
		}
		cluster.KubeAPILoadBalancer = loadBalancer
		cluster.KubeAPILoadBalancerNetwork = loadBalancerNetwork
		cluster.KubeAPILoadBalancerService = service
		cluster.KubeAPILoadBalancerTargets = targets
		cluster.Endpoint = loadBalancer.Ipv4
	} else if len(controlPlaneNodes) > 0 {
		cluster.Endpoint = controlPlaneNodes[0].PublicIP
	} else {
		cluster.Endpoint = pulumi.String("").ToStringOutput()
	}

	cluster.Kubeconfig = pulumi.String("").ToStringOutput()
	cluster.Talosconfig = pulumi.String("").ToStringOutput()

	if err := ctx.RegisterResourceOutputs(cluster, pulumi.Map{
		"endpoint":               cluster.Endpoint,
		"kubeconfig":             cluster.Kubeconfig,
		"controlPlaneNodeNames":  pulumi.ToStringArray(nodeNames(cluster.ControlPlaneNodes)),
		"workerNodeNames":        pulumi.ToStringArray(nodeNames(cluster.WorkerNodes)),
		"requiredArchitectures":  pulumi.ToStringArray(cluster.RequiredArchitectures),
		"talosImageReferences":   pulumi.ToStringMap(cluster.TalosImages),
		"controlPlanePrivateIps": pulumi.ToStringArray(nodePrivateIPs(cluster.ControlPlaneNodes)),
		"workerPrivateIps":       pulumi.ToStringArray(nodePrivateIPs(cluster.WorkerNodes)),
	}); err != nil {
		return nil, err
	}

	return cluster, nil
}

func newSubnet(ctx *pulumi.Context, name string, network *hcloud.Network, zone string, cidr string, opts ...pulumi.ResourceOption) (*hcloud.NetworkSubnet, error) {
	return hcloud.NewNetworkSubnet(ctx, name, &hcloud.NetworkSubnetArgs{
		NetworkId:   network.ID().ApplyT(resourceIDToInt).(pulumi.IntOutput),
		Type:        pulumi.String("cloud"),
		NetworkZone: pulumi.String(zone),
		IpRange:     pulumi.String(cidr),
	}, opts...)
}

func validatePulumiArgs(args ClusterArgs) error {
	for _, cidr := range args.Access.AllowedCIDRs {
		if cidr == "current-ip" {
			return fmt.Errorf("access.allowedCidrs: current-ip must be resolved before constructing Pulumi resources")
		}
	}

	return nil
}

func labels(clusterName string, role string) pulumi.StringMap {
	return labelsWith(clusterName, role, nil)
}

func labelsWith(clusterName string, role string, extra map[string]string) pulumi.StringMap {
	values := pulumi.StringMap{
		"cluster":    pulumi.String(clusterName),
		"managed-by": pulumi.String("platformctl"),
		"role":       pulumi.String(role),
	}
	for key, value := range extra {
		values[key] = pulumi.String(value)
	}

	return values
}

func nodeNames(nodes []Node) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}

	return names
}

func nodePrivateIPs(nodes []Node) []string {
	ips := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ips = append(ips, node.PrivateIP)
	}

	return ips
}
