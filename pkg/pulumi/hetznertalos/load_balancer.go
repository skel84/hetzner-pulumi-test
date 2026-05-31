package hetznertalos

import (
	"fmt"

	"github.com/pulumi/pulumi-hcloud/sdk/go/hcloud"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func createKubeAPILoadBalancer(ctx *pulumi.Context, name string, args ClusterArgs, network *hcloud.Network, controlPlaneNodes []Node, privateIP string, opts ...pulumi.ResourceOption) (*hcloud.LoadBalancer, *hcloud.LoadBalancerNetwork, *hcloud.LoadBalancerService, []*hcloud.LoadBalancerTarget, error) {
	loadBalancer, err := hcloud.NewLoadBalancer(ctx, name+"-kube-api-lb", &hcloud.LoadBalancerArgs{
		Name:             pulumi.StringPtr(name + "-kube-api"),
		LoadBalancerType: pulumi.String("lb11"),
		NetworkZone:      pulumi.StringPtr(args.Region),
		Labels:           labels(args.ClusterName, "kube-api"),
	}, opts...)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	loadBalancerNetwork, err := hcloud.NewLoadBalancerNetwork(ctx, name+"-kube-api-lb-network", &hcloud.LoadBalancerNetworkArgs{
		LoadBalancerId:        loadBalancer.ID().ApplyT(resourceIDToInt).(pulumi.IntOutput),
		NetworkId:             network.ID().ApplyT(resourceIDToIntPtr).(pulumi.IntPtrOutput),
		Ip:                    pulumi.StringPtr(privateIP),
		EnablePublicInterface: pulumi.BoolPtr(true),
	}, append(opts, pulumi.Parent(loadBalancer))...)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	service, err := hcloud.NewLoadBalancerService(ctx, name+"-kube-api-service", &hcloud.LoadBalancerServiceArgs{
		LoadBalancerId:  loadBalancer.ID(),
		Protocol:        pulumi.String("tcp"),
		ListenPort:      pulumi.IntPtr(6443),
		DestinationPort: pulumi.IntPtr(6443),
	}, append(opts, pulumi.Parent(loadBalancer))...)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	targets := make([]*hcloud.LoadBalancerTarget, 0, len(controlPlaneNodes))
	for index, node := range controlPlaneNodes {
		target, err := hcloud.NewLoadBalancerTarget(ctx, fmt.Sprintf("%s-kube-api-target-%d", name, index), &hcloud.LoadBalancerTargetArgs{
			LoadBalancerId: loadBalancer.ID().ApplyT(resourceIDToInt).(pulumi.IntOutput),
			Type:           pulumi.String("server"),
			ServerId:       node.Server.ID().ApplyT(resourceIDToIntPtr).(pulumi.IntPtrOutput),
			UsePrivateIp:   pulumi.BoolPtr(true),
		}, append(opts, pulumi.Parent(loadBalancer), pulumi.DependsOn([]pulumi.Resource{loadBalancerNetwork, node.NetworkAttachment}))...)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		targets = append(targets, target)
	}

	return loadBalancer, loadBalancerNetwork, service, targets, nil
}

func resourceIDToIntPtr(id pulumi.ID) *int {
	value := resourceIDToInt(id)
	return &value
}
