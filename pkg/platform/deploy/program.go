package deploy

import (
	"github.com/francesco/hetzner_pulumi/pkg/platform/bootstrap"
	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
	"github.com/francesco/hetzner_pulumi/pkg/pulumi/bootstrapk8s"
	"github.com/francesco/hetzner_pulumi/pkg/pulumi/hetznertalos"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func PulumiProgram(env config.EnvironmentSpec, imageRefs map[string]string, hcloudToken string, pulumiConfigPassphrase string) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		args := hetznertalos.ClusterArgsFromEnvironment(env)
		args.TalosImages = copyStringMap(imageRefs)
		cluster, err := hetznertalos.NewCluster(ctx, env.Cluster.Name, args)
		if err != nil {
			return err
		}
		if err := configureTalos(ctx, env.Cluster.Name, cluster, args); err != nil {
			return err
		}
		if args.Packages.ClusterBaseline {
			if _, err := bootstrapk8s.NewBootstrap(ctx, env.Cluster.Name+"-bootstrap", bootstrapk8s.Args{
				ClusterName:            args.ClusterName,
				Kubeconfig:             cluster.Kubeconfig,
				HCloudToken:            pulumi.ToSecret(pulumi.String(hcloudToken)).(pulumi.StringOutput),
				PulumiConfigPassphrase: pulumi.ToSecret(pulumi.String(pulumiConfigPassphrase)).(pulumi.StringOutput),
				InstallCilium:          true,
				InstallArgoCD:          args.Packages.GitOpsControlPlane,
				InstallPulumiOperator:  args.Packages.GitOpsControlPlane,
				GitOpsRoot: bootstrapk8s.GitOpsRootSpec{
					RepoURL:        env.GitOps.RepoURL,
					TargetRevision: env.GitOps.TargetRevision,
					RootPath:       env.GitOps.RootPath,
				},
			}, pulumi.Parent(cluster)); err != nil {
				return err
			}
		}

		ctx.Export("endpoint", cluster.Endpoint)
		ctx.Export("kubeconfig", pulumi.ToSecret(cluster.Kubeconfig))
		ctx.Export("talosconfig", pulumi.ToSecret(cluster.Talosconfig))
		ctx.Export("controlPlaneNodeNames", pulumi.ToStringArray(nodeNames(cluster.ControlPlaneNodes)))
		ctx.Export("workerNodeNames", pulumi.ToStringArray(nodeNames(cluster.WorkerNodes)))
		ctx.Export("requiredArchitectures", pulumi.ToStringArray(cluster.RequiredArchitectures))

		return nil
	}
}

func configureTalos(ctx *pulumi.Context, name string, cluster *hetznertalos.Cluster, args hetznertalos.ClusterArgs) error {
	patch, err := bootstrap.TalosConfigPatch(manifestProfileForCluster(args))
	if err != nil {
		return err
	}

	lifecycle := hetznertalos.NewPulumiverseLifecycle()
	clusterEndpoint := pulumi.Sprintf("https://%s:6443", cluster.Endpoint)
	generated, err := lifecycle.Generate(ctx, name, hetznertalos.TalosGenerateArgs{
		ClusterName:               args.ClusterName,
		ClusterEndpoint:           clusterEndpoint,
		TalosVersion:              args.TalosVersion,
		KubernetesVersion:         args.KubernetesVersion,
		ControlPlaneConfigPatches: []string{patch},
		WorkerConfigPatches:       []string{patch},
	}, pulumi.Parent(cluster))
	if err != nil {
		return err
	}

	access := hetznertalos.TalosClusterAccessArgs{
		ClusterName:       args.ClusterName,
		Endpoint:          cluster.Endpoint,
		ControlPlaneNodes: talosNodes(cluster.ControlPlaneNodes),
		WorkerNodes:       talosNodes(cluster.WorkerNodes),
	}

	applied, err := lifecycle.Apply(ctx, name, generated, access, pulumi.Parent(cluster))
	if err != nil {
		return err
	}

	kubeconfig, err := lifecycle.Kubeconfig(ctx, name, generated, access, pulumi.Parent(cluster), pulumi.DependsOn(applied.Resources()))
	if err != nil {
		return err
	}
	talosconfig := lifecycle.Talosconfig(ctx, generated, access)
	lifecycle.CheckHealth(ctx, generated, access)

	cluster.Kubeconfig = kubeconfig.KubeconfigRaw
	cluster.Talosconfig = talosconfig.TalosConfig()

	return nil
}

func manifestProfileForCluster(args hetznertalos.ClusterArgs) bootstrap.ManifestProfile {
	profile := bootstrap.DefaultManifestProfile()
	profile.AllowSchedulingOnControlPlanes = nodePoolCount(args.WorkerPools) == 0

	return profile
}

func nodePoolCount(pools []hetznertalos.NodePoolSpec) int {
	count := 0
	for _, pool := range pools {
		count += pool.Count
	}

	return count
}

func nodeNames(nodes []hetznertalos.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}

	return names
}

func talosNodes(nodes []hetznertalos.Node) []hetznertalos.TalosNode {
	talosNodes := make([]hetznertalos.TalosNode, 0, len(nodes))
	for _, node := range nodes {
		talosNodes = append(talosNodes, hetznertalos.TalosNode{
			Name:       node.Name,
			Endpoint:   node.PublicIP,
			InternalIP: pulumi.String(node.PrivateIP),
		})
	}

	return talosNodes
}
