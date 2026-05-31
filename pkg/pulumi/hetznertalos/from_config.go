package hetznertalos

import "github.com/francesco/hetzner_pulumi/pkg/platform/config"

func ClusterArgsFromEnvironment(env config.EnvironmentSpec) ClusterArgs {
	return ClusterArgs{
		ClusterName:       env.Cluster.Name,
		Region:            env.Cluster.Region,
		TalosVersion:      env.Cluster.TalosVersion,
		KubernetesVersion: env.Cluster.KubernetesVersion,
		NetworkCIDR:       env.Network.CIDR,
		Access: AccessArgs{
			Mode:         string(env.Access.Mode),
			AllowedCIDRs: copyStrings(env.Access.AllowedCIDRs),
		},
		ControlPlanePools: copyNodePools(env.NodePools.ControlPlane),
		WorkerPools:       copyNodePools(env.NodePools.Workers),
		Packages: PackageProfile{
			ClusterBaseline:    env.Packages.ClusterBaseline,
			GitOpsControlPlane: env.Packages.GitOpsControlPlane,
			Secrets:            env.Packages.Secrets,
			CertsDNS:           env.Packages.CertsDNS,
		},
	}
}

func copyNodePools(pools []config.NodePoolSpec) []NodePoolSpec {
	copied := make([]NodePoolSpec, 0, len(pools))
	for _, pool := range pools {
		copied = append(copied, NodePoolSpec{
			Name:         pool.Name,
			Type:         pool.Type,
			Location:     pool.Location,
			Architecture: pool.Architecture,
			Count:        pool.Count,
		})
	}

	return copied
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}
