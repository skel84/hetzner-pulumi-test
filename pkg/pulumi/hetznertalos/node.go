package hetznertalos

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-hcloud/sdk/go/hcloud"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type nodePoolBuildArgs struct {
	ClusterName    string
	Role           string
	SubnetCIDR     string
	Images         map[string]string
	Network        *hcloud.Network
	Firewall       *hcloud.Firewall
	PlacementGroup *hcloud.PlacementGroup
	StartIndex     int
	Pools          []NodePoolSpec
}

func createNodePoolResources(ctx *pulumi.Context, args nodePoolBuildArgs, opts ...pulumi.ResourceOption) ([]Node, error) {
	nodes := []Node{}
	globalIndex := args.StartIndex

	for _, pool := range args.Pools {
		image := args.Images[pool.Architecture]
		for poolIndex := 0; poolIndex < pool.Count; poolIndex++ {
			nodeName := fmt.Sprintf("%s-%s-%d", args.ClusterName, pool.Name, poolIndex)
			privateIP, err := privateIP(args.SubnetCIDR, 10+globalIndex)
			if err != nil {
				return nil, err
			}

			server, err := hcloud.NewServer(ctx, nodeName, &hcloud.ServerArgs{
				Name:       pulumi.StringPtr(nodeName),
				Image:      pulumi.StringPtr(image),
				ServerType: pulumi.String(pool.Type),
				Location:   pulumi.StringPtr(pool.Location),
				Labels: labelsWith(args.ClusterName, args.Role, map[string]string{
					"node-pool":    pool.Name,
					"architecture": pool.Architecture,
				}),
				PublicNets: hcloud.ServerPublicNetArray{
					&hcloud.ServerPublicNetArgs{
						Ipv4Enabled: pulumi.Bool(true),
						Ipv6Enabled: pulumi.Bool(true),
					},
				},
				ShutdownBeforeDeletion: pulumi.BoolPtr(true),
				PlacementGroupId:       placementGroupID(args.PlacementGroup),
				FirewallIds: pulumi.IntArray{
					args.Firewall.ID().ApplyT(resourceIDToInt).(pulumi.IntOutput),
				},
			}, opts...)
			if err != nil {
				return nil, err
			}

			attachment, err := hcloud.NewServerNetwork(ctx, nodeName+"-network", &hcloud.ServerNetworkArgs{
				ServerId: server.ID().ApplyT(resourceIDToInt).(pulumi.IntOutput),
				NetworkId: args.Network.ID().ApplyT(func(id pulumi.ID) *int {
					value := resourceIDToInt(id)
					return &value
				}).(pulumi.IntPtrOutput),
				Ip: pulumi.StringPtr(privateIP),
			}, append(opts, pulumi.Parent(server))...)
			if err != nil {
				return nil, err
			}

			nodes = append(nodes, Node{
				Server:            server,
				NetworkAttachment: attachment,
				Name:              nodeName,
				Role:              args.Role,
				Location:          pool.Location,
				Architecture:      pool.Architecture,
				PrivateIP:         privateIP,
				PublicIP:          server.Ipv4Address,
			})
			globalIndex++
		}
	}

	return nodes, nil
}

func placementGroupID(placementGroup *hcloud.PlacementGroup) pulumi.IntPtrInput {
	if placementGroup == nil {
		return nil
	}

	return placementGroup.ID().ApplyT(func(id pulumi.ID) *int {
		value := resourceIDToInt(id)
		return &value
	}).(pulumi.IntPtrOutput)
}

func resourceIDToInt(id pulumi.ID) int {
	converted, err := strconv.Atoi(string(id))
	if err != nil {
		return 0
	}

	return converted
}
