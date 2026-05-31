package hetznertalos

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	talosclient "github.com/pulumiverse/pulumi-talos/sdk/go/talos/client"
	taloscluster "github.com/pulumiverse/pulumi-talos/sdk/go/talos/cluster"
	"github.com/pulumiverse/pulumi-talos/sdk/go/talos/machine"
)

type TalosLifecycle interface {
	Generate(ctx *pulumi.Context, name string, args TalosGenerateArgs, opts ...pulumi.ResourceOption) (*GeneratedTalosCluster, error)
	Apply(ctx *pulumi.Context, name string, generated *GeneratedTalosCluster, args TalosClusterAccessArgs, opts ...pulumi.ResourceOption) (*AppliedTalosCluster, error)
	Kubeconfig(ctx *pulumi.Context, name string, generated *GeneratedTalosCluster, args TalosClusterAccessArgs, opts ...pulumi.ResourceOption) (*taloscluster.Kubeconfig, error)
	Talosconfig(ctx *pulumi.Context, generated *GeneratedTalosCluster, args TalosClusterAccessArgs, opts ...pulumi.InvokeOption) talosclient.GetConfigurationResultOutput
	CheckHealth(ctx *pulumi.Context, generated *GeneratedTalosCluster, args TalosClusterAccessArgs, opts ...pulumi.InvokeOption) taloscluster.GetHealthResultOutput
}

type TalosGenerateArgs struct {
	ClusterName               string
	ClusterEndpoint           pulumi.StringInput
	TalosVersion              string
	KubernetesVersion         string
	ControlPlaneConfigPatches []string
	WorkerConfigPatches       []string
}

type GeneratedTalosCluster struct {
	Secrets                   *machine.Secrets
	ControlPlaneConfiguration machine.GetConfigurationResultOutput
	WorkerConfiguration       machine.GetConfigurationResultOutput
}

type TalosClusterAccessArgs struct {
	ClusterName       string
	Endpoint          pulumi.StringInput
	ControlPlaneNodes []TalosNode
	WorkerNodes       []TalosNode
}

type TalosNode struct {
	Name       string
	Endpoint   pulumi.StringInput
	InternalIP pulumi.StringInput
}

type AppliedTalosCluster struct {
	InitialControlPlaneApply     *machine.ConfigurationApply
	Bootstrap                    *machine.Bootstrap
	RemainingControlPlaneApplies []*machine.ConfigurationApply
	WorkerApplies                []*machine.ConfigurationApply
}

type PulumiverseLifecycle struct{}

func NewPulumiverseLifecycle() *PulumiverseLifecycle {
	return &PulumiverseLifecycle{}
}

func (PulumiverseLifecycle) Generate(ctx *pulumi.Context, name string, args TalosGenerateArgs, opts ...pulumi.ResourceOption) (*GeneratedTalosCluster, error) {
	secrets, err := machine.NewSecrets(ctx, name+"-talos-secrets", &machine.SecretsArgs{
		TalosVersion: pulumi.StringPtr(args.TalosVersion),
	}, opts...)
	if err != nil {
		return nil, err
	}

	controlPlaneConfiguration := machine.GetConfigurationOutput(ctx, machine.GetConfigurationOutputArgs{
		ClusterName:       pulumi.String(args.ClusterName),
		ClusterEndpoint:   args.ClusterEndpoint,
		MachineType:       pulumi.String("controlplane"),
		MachineSecrets:    secrets.MachineSecrets,
		TalosVersion:      pulumi.StringPtr(args.TalosVersion),
		KubernetesVersion: pulumi.StringPtr(args.KubernetesVersion),
		ConfigPatches:     pulumi.ToStringArray(args.ControlPlaneConfigPatches),
	})

	workerConfiguration := machine.GetConfigurationOutput(ctx, machine.GetConfigurationOutputArgs{
		ClusterName:       pulumi.String(args.ClusterName),
		ClusterEndpoint:   args.ClusterEndpoint,
		MachineType:       pulumi.String("worker"),
		MachineSecrets:    secrets.MachineSecrets,
		TalosVersion:      pulumi.StringPtr(args.TalosVersion),
		KubernetesVersion: pulumi.StringPtr(args.KubernetesVersion),
		ConfigPatches:     pulumi.ToStringArray(args.WorkerConfigPatches),
	})

	return &GeneratedTalosCluster{
		Secrets:                   secrets,
		ControlPlaneConfiguration: controlPlaneConfiguration,
		WorkerConfiguration:       workerConfiguration,
	}, nil
}

func (PulumiverseLifecycle) Apply(ctx *pulumi.Context, name string, generated *GeneratedTalosCluster, args TalosClusterAccessArgs, opts ...pulumi.ResourceOption) (*AppliedTalosCluster, error) {
	if err := requireControlPlaneNodes(args); err != nil {
		return nil, err
	}

	initialControlPlane := args.ControlPlaneNodes[0]
	initialApply, err := newConfigurationApply(ctx, name+"-talos-apply-control-plane-0", generated.Secrets.ClientConfiguration, generated.ControlPlaneConfiguration.MachineConfiguration(), initialControlPlane.Endpoint, opts...)
	if err != nil {
		return nil, err
	}

	bootstrap, err := machine.NewBootstrap(ctx, name+"-talos-bootstrap", &machine.BootstrapArgs{
		ClientConfiguration: generated.Secrets.ClientConfiguration.ToClientConfigurationPtrOutput(),
		Node:                initialControlPlane.Endpoint,
		Endpoint:            stringInputPtr(args.Endpoint),
	}, append(opts, pulumi.DependsOn([]pulumi.Resource{initialApply}))...)
	if err != nil {
		return nil, err
	}

	applied := &AppliedTalosCluster{
		InitialControlPlaneApply: initialApply,
		Bootstrap:                bootstrap,
	}

	for index, node := range args.ControlPlaneNodes[1:] {
		apply, err := newConfigurationApply(ctx, name+"-talos-apply-control-plane-"+strconv.Itoa(index+1), generated.Secrets.ClientConfiguration, generated.ControlPlaneConfiguration.MachineConfiguration(), node.Endpoint, append(opts, pulumi.DependsOn([]pulumi.Resource{bootstrap}))...)
		if err != nil {
			return nil, err
		}
		applied.RemainingControlPlaneApplies = append(applied.RemainingControlPlaneApplies, apply)
	}

	for index, node := range args.WorkerNodes {
		apply, err := newConfigurationApply(ctx, name+"-talos-apply-worker-"+strconv.Itoa(index), generated.Secrets.ClientConfiguration, generated.WorkerConfiguration.MachineConfiguration(), node.Endpoint, append(opts, pulumi.DependsOn([]pulumi.Resource{bootstrap}))...)
		if err != nil {
			return nil, err
		}
		applied.WorkerApplies = append(applied.WorkerApplies, apply)
	}

	return applied, nil
}

func (PulumiverseLifecycle) Kubeconfig(ctx *pulumi.Context, name string, generated *GeneratedTalosCluster, args TalosClusterAccessArgs, opts ...pulumi.ResourceOption) (*taloscluster.Kubeconfig, error) {
	if err := requireControlPlaneNodes(args); err != nil {
		return nil, err
	}

	return taloscluster.NewKubeconfig(ctx, name+"-kubeconfig", &taloscluster.KubeconfigArgs{
		ClientConfiguration: kubeconfigClientConfiguration(generated.Secrets.ClientConfiguration),
		Node:                args.ControlPlaneNodes[0].Endpoint,
		Endpoint:            stringInputPtr(args.Endpoint),
	}, opts...)
}

func requireControlPlaneNodes(args TalosClusterAccessArgs) error {
	if len(args.ControlPlaneNodes) == 0 {
		return fmt.Errorf("controlPlaneNodes: at least one control plane node is required")
	}

	return nil
}

func (PulumiverseLifecycle) Talosconfig(ctx *pulumi.Context, generated *GeneratedTalosCluster, args TalosClusterAccessArgs, opts ...pulumi.InvokeOption) talosclient.GetConfigurationResultOutput {
	return talosclient.GetConfigurationOutput(ctx, talosclient.GetConfigurationOutputArgs{
		ClientConfiguration: talosconfigClientConfiguration(generated.Secrets.ClientConfiguration),
		ClusterName:         pulumi.String(args.ClusterName),
		Endpoints:           pulumi.StringArray{args.Endpoint},
		Nodes:               talosNodeEndpoints(args.ControlPlaneNodes, args.WorkerNodes),
	}, opts...)
}

func (PulumiverseLifecycle) CheckHealth(ctx *pulumi.Context, generated *GeneratedTalosCluster, args TalosClusterAccessArgs, opts ...pulumi.InvokeOption) taloscluster.GetHealthResultOutput {
	return taloscluster.GetHealthOutput(ctx, taloscluster.GetHealthOutputArgs{
		ClientConfiguration:  healthClientConfiguration(generated.Secrets.ClientConfiguration),
		Endpoints:            pulumi.StringArray{args.Endpoint},
		ControlPlaneNodes:    talosNodeInternalIPs(args.ControlPlaneNodes, nil),
		WorkerNodes:          talosNodeInternalIPs(args.WorkerNodes, nil),
		SkipKubernetesChecks: pulumi.BoolPtr(true),
		Timeouts: taloscluster.GetHealthTimeoutsArgs{
			Read: pulumi.StringPtr("5m"),
		},
	}, opts...)
}

func newConfigurationApply(ctx *pulumi.Context, name string, clientConfiguration machine.ClientConfigurationOutput, machineConfiguration pulumi.StringOutput, node pulumi.StringInput, opts ...pulumi.ResourceOption) (*machine.ConfigurationApply, error) {
	return machine.NewConfigurationApply(ctx, name, &machine.ConfigurationApplyArgs{
		ClientConfiguration:       clientConfiguration.ToClientConfigurationPtrOutput(),
		MachineConfigurationInput: stringPtrOutput(machineConfiguration),
		Node:                      node,
		Endpoint:                  stringInputPtr(node),
	}, opts...)
}

func stringPtrOutput(value pulumi.StringOutput) pulumi.StringPtrOutput {
	return value.ApplyT(func(text string) *string {
		return &text
	}).(pulumi.StringPtrOutput)
}

func stringInputPtr(value pulumi.StringInput) pulumi.StringPtrInput {
	return pulumi.Sprintf("%s", value).ToStringPtrOutput()
}

func kubeconfigClientConfiguration(input machine.ClientConfigurationOutput) taloscluster.KubeconfigClientConfigurationOutput {
	return input.ApplyT(func(value machine.ClientConfiguration) taloscluster.KubeconfigClientConfiguration {
		return taloscluster.KubeconfigClientConfiguration{
			CaCertificate:     value.CaCertificate,
			ClientCertificate: value.ClientCertificate,
			ClientKey:         value.ClientKey,
		}
	}).(taloscluster.KubeconfigClientConfigurationOutput)
}

func talosconfigClientConfiguration(input machine.ClientConfigurationOutput) talosclient.GetConfigurationClientConfigurationOutput {
	return input.ApplyT(func(value machine.ClientConfiguration) talosclient.GetConfigurationClientConfiguration {
		return talosclient.GetConfigurationClientConfiguration{
			CaCertificate:     value.CaCertificate,
			ClientCertificate: value.ClientCertificate,
			ClientKey:         value.ClientKey,
		}
	}).(talosclient.GetConfigurationClientConfigurationOutput)
}

func healthClientConfiguration(input machine.ClientConfigurationOutput) taloscluster.GetHealthClientConfigurationOutput {
	return input.ApplyT(func(value machine.ClientConfiguration) taloscluster.GetHealthClientConfiguration {
		return taloscluster.GetHealthClientConfiguration{
			CaCertificate:     value.CaCertificate,
			ClientCertificate: value.ClientCertificate,
			ClientKey:         value.ClientKey,
		}
	}).(taloscluster.GetHealthClientConfigurationOutput)
}

func talosNodeEndpoints(first []TalosNode, second []TalosNode) pulumi.StringArray {
	endpoints := make(pulumi.StringArray, 0, len(first)+len(second))
	for _, node := range first {
		endpoints = append(endpoints, node.Endpoint)
	}
	for _, node := range second {
		endpoints = append(endpoints, node.Endpoint)
	}

	return endpoints
}

func talosNodeInternalIPs(first []TalosNode, second []TalosNode) pulumi.StringArray {
	endpoints := make(pulumi.StringArray, 0, len(first)+len(second))
	for _, node := range first {
		endpoints = append(endpoints, node.InternalIP)
	}
	for _, node := range second {
		endpoints = append(endpoints, node.InternalIP)
	}

	return endpoints
}

func (applied *AppliedTalosCluster) Resources() []pulumi.Resource {
	resources := []pulumi.Resource{applied.InitialControlPlaneApply, applied.Bootstrap}
	for _, apply := range applied.RemainingControlPlaneApplies {
		resources = append(resources, apply)
	}
	for _, apply := range applied.WorkerApplies {
		resources = append(resources, apply)
	}

	return resources
}
