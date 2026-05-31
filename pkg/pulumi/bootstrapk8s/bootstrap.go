package bootstrapk8s

import (
	"fmt"
	"regexp"
	"strings"

	kubernetes "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	apiextensions "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	networkingv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/networking/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const bootstrapType = "platform:bootstrapk8s:Bootstrap"

const (
	DefaultCiliumChart                = "https://helm.cilium.io/cilium-1.19.4.tgz"
	DefaultCiliumChartVersion         = "1.19.4"
	DefaultArgoCDChartVersion         = "9.5.17"
	DefaultArgoCDChart                = "https://github.com/argoproj/argo-helm/releases/download/argo-cd-" + DefaultArgoCDChartVersion + "/argo-cd-" + DefaultArgoCDChartVersion + ".tgz"
	DefaultPulumiOperatorChart        = "oci://ghcr.io/pulumi/helm-charts/pulumi-kubernetes-operator"
	DefaultPulumiOperatorChartVersion = "2.7.0"
	DefaultGitOpsTargetRevision       = "main"
	DefaultGitOpsRootPath             = "gitops/root"
)

var kubernetesNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

type Args struct {
	ClusterName           string
	Kubeconfig            pulumi.StringInput
	HCloudToken           pulumi.StringInput
	Namespaces            []NamespaceSpec
	InstallCilium         bool
	InstallArgoCD         bool
	InstallPulumiOperator bool
	GitOpsRoot            GitOpsRootSpec
}

type NamespaceSpec struct {
	Name               string
	Purpose            string
	DefaultDenyIngress bool
}

type GitOpsRootSpec struct {
	RepoURL        string
	TargetRevision string
	RootPath       string
}

type Bootstrap struct {
	pulumi.ResourceState

	Provider        *kubernetes.Provider
	HCloudSecret    *corev1.Secret
	Namespaces      []*corev1.Namespace
	NetworkPolicies []*networkingv1.NetworkPolicy
	Cilium          *helmv3.Release
	ArgoCD          *helmv3.Release
	PulumiOperator  *helmv3.Release
	RootApplication *apiextensions.CustomResource
}

func NewBootstrap(ctx *pulumi.Context, name string, args Args, opts ...pulumi.ResourceOption) (*Bootstrap, error) {
	namespaces := args.Namespaces
	if len(namespaces) == 0 {
		namespaces = DefaultNamespaces()
	}
	if err := validateArgs(Args{
		ClusterName: args.ClusterName,
		Kubeconfig:  args.Kubeconfig,
		Namespaces:  namespaces,
	}); err != nil {
		return nil, err
	}

	bootstrap := &Bootstrap{}
	if err := ctx.RegisterComponentResource(bootstrapType, name, bootstrap, opts...); err != nil {
		return nil, err
	}

	provider, err := kubernetes.NewProvider(ctx, name+"-k8s", &kubernetes.ProviderArgs{
		Kubeconfig:            args.Kubeconfig,
		ClusterIdentifier:     pulumi.String(args.ClusterName),
		EnableServerSideApply: pulumi.Bool(true),
		KubeClientSettings: kubernetes.KubeClientSettingsArgs{
			Timeout: pulumi.IntPtr(180),
		},
	}, pulumi.Parent(bootstrap))
	if err != nil {
		return nil, err
	}
	bootstrap.Provider = provider

	childOpts := []pulumi.ResourceOption{
		pulumi.Parent(bootstrap),
		pulumi.Provider(provider),
	}

	if args.HCloudToken != nil {
		secret, err := corev1.NewSecret(ctx, name+"-hcloud-secret", &corev1.SecretArgs{
			Metadata: metav1.ObjectMetaArgs{
				Name:      pulumi.String("hcloud"),
				Namespace: pulumi.String("kube-system"),
				Labels:    baseLabels(args.ClusterName, "hetzner cloud controller manager"),
			},
			StringData: pulumi.StringMap{
				"token": args.HCloudToken,
			},
			Type: pulumi.StringPtr("Opaque"),
		}, childOpts...)
		if err != nil {
			return nil, err
		}
		bootstrap.HCloudSecret = secret
	}

	for _, namespace := range namespaces {
		resourceName := name + "-" + namespace.Name
		createdNamespace, err := corev1.NewNamespace(ctx, resourceName, &corev1.NamespaceArgs{
			Metadata: metav1.ObjectMetaArgs{
				Name:   pulumi.String(namespace.Name),
				Labels: namespaceLabels(args.ClusterName, namespace),
			},
		}, childOpts...)
		if err != nil {
			return nil, err
		}
		bootstrap.Namespaces = append(bootstrap.Namespaces, createdNamespace)

		if namespace.DefaultDenyIngress {
			networkPolicy, err := networkingv1.NewNetworkPolicy(ctx, resourceName+"-deny-ingress", &networkingv1.NetworkPolicyArgs{
				Metadata: metav1.ObjectMetaArgs{
					Name:      pulumi.String("default-deny-ingress"),
					Namespace: pulumi.String(namespace.Name),
					Labels:    baseLabels(args.ClusterName, namespace.Purpose),
				},
				Spec: networkingv1.NetworkPolicySpecArgs{
					PodSelector: metav1.LabelSelectorArgs{},
					PolicyTypes: pulumi.StringArray{pulumi.String("Ingress")},
				},
			}, childOpts...)
			if err != nil {
				return nil, err
			}
			bootstrap.NetworkPolicies = append(bootstrap.NetworkPolicies, networkPolicy)
		}
	}

	ciliumOpts := append([]pulumi.ResourceOption{}, childOpts...)
	if args.InstallCilium {
		cilium, err := helmv3.NewRelease(ctx, name+"-cilium", &helmv3.ReleaseArgs{
			Name:            pulumi.StringPtr("cilium"),
			Chart:           pulumi.String(DefaultCiliumChart),
			Namespace:       pulumi.StringPtr("kube-system"),
			Atomic:          pulumi.BoolPtr(true),
			CreateNamespace: pulumi.BoolPtr(false),
			Timeout:         pulumi.IntPtr(600),
			Values:          ciliumValues(),
		}, ciliumOpts...)
		if err != nil {
			return nil, err
		}
		bootstrap.Cilium = cilium
	}

	releaseDependencies := namespaceResources(bootstrap.Namespaces)
	if bootstrap.Cilium != nil {
		releaseDependencies = append(releaseDependencies, bootstrap.Cilium)
	}
	releaseOpts := append([]pulumi.ResourceOption{}, childOpts...)
	releaseOpts = append(releaseOpts, pulumi.DependsOn(releaseDependencies))
	if args.InstallArgoCD {
		argocd, err := helmv3.NewRelease(ctx, name+"-argocd", &helmv3.ReleaseArgs{
			Name:            pulumi.StringPtr("argocd"),
			Chart:           pulumi.String(DefaultArgoCDChart),
			Namespace:       pulumi.StringPtr("platform-gitops"),
			Atomic:          pulumi.BoolPtr(true),
			CreateNamespace: pulumi.BoolPtr(false),
			Timeout:         pulumi.IntPtr(600),
		}, releaseOpts...)
		if err != nil {
			return nil, err
		}
		bootstrap.ArgoCD = argocd
	}
	if args.InstallPulumiOperator {
		operator, err := helmv3.NewRelease(ctx, name+"-pulumi-kubernetes-operator", &helmv3.ReleaseArgs{
			Name:            pulumi.StringPtr("pulumi-kubernetes-operator"),
			Chart:           pulumi.String(DefaultPulumiOperatorChart),
			Version:         pulumi.StringPtr(DefaultPulumiOperatorChartVersion),
			Namespace:       pulumi.StringPtr("platform-pulumi"),
			Atomic:          pulumi.BoolPtr(true),
			CreateNamespace: pulumi.BoolPtr(false),
			Timeout:         pulumi.IntPtr(600),
		}, releaseOpts...)
		if err != nil {
			return nil, err
		}
		bootstrap.PulumiOperator = operator
	}

	if gitOpsRootEnabled(args.GitOpsRoot) {
		if bootstrap.ArgoCD == nil {
			return nil, fmt.Errorf("gitops root application requires Argo CD installation")
		}
		rootApplication, err := apiextensions.NewCustomResource(ctx, name+"-root-application", &apiextensions.CustomResourceArgs{
			ApiVersion: pulumi.String("argoproj.io/v1alpha1"),
			Kind:       pulumi.String("Application"),
			Metadata: metav1.ObjectMetaArgs{
				Name:      pulumi.String("platform-root"),
				Namespace: pulumi.String("platform-gitops"),
				Labels:    baseLabels(args.ClusterName, "gitops root"),
			},
			OtherFields: gitOpsRootApplicationFields(args.GitOpsRoot),
		}, gitOpsRootOptions(childOpts, bootstrap)...)
		if err != nil {
			return nil, err
		}
		bootstrap.RootApplication = rootApplication
	}

	if err := ctx.RegisterResourceOutputs(bootstrap, pulumi.Map{
		"namespaceCount":     pulumi.Int(len(bootstrap.Namespaces)),
		"networkPolicyCount": pulumi.Int(len(bootstrap.NetworkPolicies)),
	}); err != nil {
		return nil, err
	}

	return bootstrap, nil
}

func DefaultNamespaces() []NamespaceSpec {
	return []NamespaceSpec{
		{Name: "platform-system", Purpose: "platform runtime", DefaultDenyIngress: true},
		{Name: "platform-gitops", Purpose: "gitops control plane"},
		{Name: "platform-pulumi", Purpose: "pulumi kubernetes operator"},
	}
}

func validateArgs(args Args) error {
	if strings.TrimSpace(args.ClusterName) == "" {
		return fmt.Errorf("cluster name is required")
	}
	if args.Kubeconfig == nil {
		return fmt.Errorf("kubeconfig is required")
	}

	seen := map[string]struct{}{}
	for index, namespace := range args.Namespaces {
		if strings.TrimSpace(namespace.Name) == "" {
			return fmt.Errorf("namespace[%d].name is required", index)
		}
		if len(namespace.Name) > 63 {
			return fmt.Errorf("namespace[%d].name must be 63 characters or fewer", index)
		}
		if !kubernetesNamePattern.MatchString(namespace.Name) {
			return fmt.Errorf("namespace[%d].name must contain only lowercase letters, numbers, and hyphens", index)
		}
		if _, ok := seen[namespace.Name]; ok {
			return fmt.Errorf("namespace %q is duplicated", namespace.Name)
		}
		seen[namespace.Name] = struct{}{}
	}

	return nil
}

func gitOpsRootEnabled(spec GitOpsRootSpec) bool {
	return strings.TrimSpace(spec.RepoURL) != ""
}

func gitOpsTargetRevision(spec GitOpsRootSpec) string {
	if strings.TrimSpace(spec.TargetRevision) == "" {
		return DefaultGitOpsTargetRevision
	}

	return spec.TargetRevision
}

func gitOpsRootPath(spec GitOpsRootSpec) string {
	if strings.TrimSpace(spec.RootPath) == "" {
		return DefaultGitOpsRootPath
	}

	return spec.RootPath
}

func gitOpsRootApplicationFields(spec GitOpsRootSpec) kubernetes.UntypedArgs {
	return kubernetes.UntypedArgs{
		"spec": map[string]any{
			"project": "default",
			"source": map[string]any{
				"repoURL":        spec.RepoURL,
				"targetRevision": gitOpsTargetRevision(spec),
				"path":           gitOpsRootPath(spec),
			},
			"destination": map[string]any{
				"server":    "https://kubernetes.default.svc",
				"namespace": "platform-gitops",
			},
			"syncPolicy": map[string]any{
				"automated": map[string]any{
					"prune":    true,
					"selfHeal": true,
				},
				"syncOptions": []any{"CreateNamespace=true"},
			},
		},
	}
}

func gitOpsRootOptions(base []pulumi.ResourceOption, bootstrap *Bootstrap) []pulumi.ResourceOption {
	opts := append([]pulumi.ResourceOption{}, base...)
	dependencies := []pulumi.Resource{bootstrap.ArgoCD}
	if bootstrap.PulumiOperator != nil {
		dependencies = append(dependencies, bootstrap.PulumiOperator)
	}

	return append(opts, pulumi.DependsOn(dependencies))
}

func namespaceLabels(clusterName string, namespace NamespaceSpec) pulumi.StringMap {
	return pulumi.StringMap{
		"app.kubernetes.io/managed-by":       pulumi.String("platformctl"),
		"app.kubernetes.io/part-of":          pulumi.String("hetzner-pulumi-platform"),
		"platformctl.dev/cluster":            pulumi.String(clusterName),
		"platformctl.dev/purpose":            pulumi.String(labelValue(namespace.Purpose)),
		"pod-security.kubernetes.io/enforce": pulumi.String("baseline"),
		"pod-security.kubernetes.io/audit":   pulumi.String("restricted"),
		"pod-security.kubernetes.io/warn":    pulumi.String("restricted"),
	}
}

func baseLabels(clusterName string, purpose string) pulumi.StringMap {
	return pulumi.StringMap{
		"app.kubernetes.io/managed-by": pulumi.String("platformctl"),
		"app.kubernetes.io/part-of":    pulumi.String("hetzner-pulumi-platform"),
		"platformctl.dev/cluster":      pulumi.String(clusterName),
		"platformctl.dev/purpose":      pulumi.String(labelValue(purpose)),
	}
}

func ciliumValues() pulumi.Map {
	return pulumi.Map{
		"ipam": pulumi.Map{
			"mode": pulumi.String("kubernetes"),
		},
		"kubeProxyReplacement": pulumi.Bool(false),
		"securityContext": pulumi.Map{
			"capabilities": pulumi.Map{
				"ciliumAgent": pulumi.StringArray{
					pulumi.String("CHOWN"),
					pulumi.String("KILL"),
					pulumi.String("NET_ADMIN"),
					pulumi.String("NET_RAW"),
					pulumi.String("IPC_LOCK"),
					pulumi.String("SYS_ADMIN"),
					pulumi.String("SYS_RESOURCE"),
					pulumi.String("DAC_OVERRIDE"),
					pulumi.String("FOWNER"),
					pulumi.String("SETGID"),
					pulumi.String("SETUID"),
				},
				"cleanCiliumState": pulumi.StringArray{
					pulumi.String("NET_ADMIN"),
					pulumi.String("SYS_ADMIN"),
					pulumi.String("SYS_RESOURCE"),
				},
			},
		},
		"cgroup": pulumi.Map{
			"autoMount": pulumi.Map{
				"enabled": pulumi.Bool(false),
			},
			"hostRoot": pulumi.String("/sys/fs/cgroup"),
		},
		"operator": pulumi.Map{
			"replicas": pulumi.Int(1),
		},
	}
}

func labelValue(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(strings.ToLower(value)), " ", "-")
}

func namespaceResources(namespaces []*corev1.Namespace) []pulumi.Resource {
	resources := make([]pulumi.Resource, 0, len(namespaces))
	for _, namespace := range namespaces {
		resources = append(resources, namespace)
	}

	return resources
}
