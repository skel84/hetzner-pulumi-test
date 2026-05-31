package bootstrapk8s

import (
	"strings"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func TestNewBootstrapRegistersProviderNamespacesAndPolicies(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bootstrap, err := NewBootstrap(ctx, "dev-eu-1-bootstrap", Args{
			ClusterName: "dev-eu-1",
			Kubeconfig:  pulumi.String("apiVersion: v1"),
		})
		if err != nil {
			return err
		}
		if got := len(bootstrap.Namespaces); got != 3 {
			t.Fatalf("len(Namespaces) = %d, want 3", got)
		}
		if got := len(bootstrap.NetworkPolicies); got != 1 {
			t.Fatalf("len(NetworkPolicies) = %d, want 1", got)
		}

		return nil
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	for _, name := range []string{
		"dev-eu-1-bootstrap-k8s",
		"dev-eu-1-bootstrap-platform-system",
		"dev-eu-1-bootstrap-platform-gitops",
		"dev-eu-1-bootstrap-platform-pulumi",
		"dev-eu-1-bootstrap-platform-system-deny-ingress",
	} {
		if !mocks.hasResource(name) {
			t.Fatalf("expected resource %q, got %#v", name, mocks.names())
		}
	}

	provider := mocks.resourceByName("dev-eu-1-bootstrap-k8s")
	if got := provider.inputs["kubeconfig"].StringValue(); got != "apiVersion: v1" {
		t.Fatalf("provider kubeconfig = %q, want literal kubeconfig", got)
	}
	if got := provider.inputs["clusterIdentifier"].StringValue(); got != "dev-eu-1" {
		t.Fatalf("provider clusterIdentifier = %q, want dev-eu-1", got)
	}
	if got := provider.inputs["enableServerSideApply"].BoolValue(); !got {
		t.Fatal("provider enableServerSideApply = false, want true")
	}
	kubeClientSettings := provider.inputs["kubeClientSettings"].ObjectValue()
	if got := kubeClientSettings["timeout"].NumberValue(); got != 180 {
		t.Fatalf("provider kube client timeout = %v, want 180", got)
	}

	namespace := mocks.resourceByName("dev-eu-1-bootstrap-platform-system")
	metadata := namespace.inputs["metadata"].ObjectValue()
	if got := metadata["name"].StringValue(); got != "platform-system" {
		t.Fatalf("namespace metadata.name = %q, want platform-system", got)
	}
	labels := metadata["labels"].ObjectValue()
	if got := labels["pod-security.kubernetes.io/enforce"].StringValue(); got != "baseline" {
		t.Fatalf("pod security enforce label = %q, want baseline", got)
	}
	if got := labels["platformctl.dev/cluster"].StringValue(); got != "dev-eu-1" {
		t.Fatalf("cluster label = %q, want dev-eu-1", got)
	}
	if got := labels["platformctl.dev/purpose"].StringValue(); got != "platform-runtime" {
		t.Fatalf("purpose label = %q, want platform-runtime", got)
	}

	networkPolicy := mocks.resourceByName("dev-eu-1-bootstrap-platform-system-deny-ingress")
	npMetadata := networkPolicy.inputs["metadata"].ObjectValue()
	if got := npMetadata["namespace"].StringValue(); got != "platform-system" {
		t.Fatalf("network policy namespace = %q, want platform-system", got)
	}
	spec := networkPolicy.inputs["spec"].ObjectValue()
	if got := spec["policyTypes"].ArrayValue()[0].StringValue(); got != "Ingress" {
		t.Fatalf("policyTypes[0] = %q, want Ingress", got)
	}
	if !spec["podSelector"].IsObject() {
		t.Fatalf("podSelector = %#v, want object", spec["podSelector"])
	}
	for _, name := range []string{"dev-eu-1-bootstrap-platform-gitops-deny-ingress", "dev-eu-1-bootstrap-platform-pulumi-deny-ingress"} {
		if mocks.hasResource(name) {
			t.Fatalf("%s should wait until allow rules exist", name)
		}
	}
}

func TestNewBootstrapAcceptsCustomNamespaces(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := NewBootstrap(ctx, "dev-eu-1-bootstrap", Args{
			ClusterName: "dev-eu-1",
			Kubeconfig:  pulumi.String("apiVersion: v1"),
			Namespaces: []NamespaceSpec{
				{Name: "platform-system", Purpose: "platform runtime", DefaultDenyIngress: true},
				{Name: "observability", Purpose: "observability", DefaultDenyIngress: true},
			},
		})
		return err
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	if !mocks.hasResource("dev-eu-1-bootstrap-observability") {
		t.Fatalf("expected observability namespace, got %#v", mocks.names())
	}
	namespace := mocks.resourceByName("dev-eu-1-bootstrap-observability")
	labels := namespace.inputs["metadata"].ObjectValue()["labels"].ObjectValue()
	if got := labels["platformctl.dev/purpose"].StringValue(); got != "observability" {
		t.Fatalf("purpose label = %q, want observability", got)
	}
	if !mocks.hasResource("dev-eu-1-bootstrap-observability-deny-ingress") {
		t.Fatalf("expected observability default-deny policy, got %#v", mocks.names())
	}
}

func TestNewBootstrapRegistersGitOpsReleases(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bootstrap, err := NewBootstrap(ctx, "dev-eu-1-bootstrap", Args{
			ClusterName:            "dev-eu-1",
			Kubeconfig:             pulumi.String("apiVersion: v1"),
			PulumiConfigPassphrase: pulumi.String("secret-passphrase"),
			InstallArgoCD:          true,
			InstallPulumiOperator:  true,
		})
		if err != nil {
			return err
		}
		if bootstrap.ArgoCD == nil {
			t.Fatal("ArgoCD release = nil, want release")
		}
		if bootstrap.PulumiOperator == nil {
			t.Fatal("PulumiOperator release = nil, want release")
		}

		return nil
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	for _, name := range []string{
		"dev-eu-1-bootstrap-argocd",
		"dev-eu-1-bootstrap-pulumi-kubernetes-operator",
		"dev-eu-1-bootstrap-pulumi-kubernetes-operator-env",
		"dev-eu-1-bootstrap-pulumi-kubernetes-operator-auth-delegator",
	} {
		if !mocks.hasResource(name) {
			t.Fatalf("expected resource %q, got %#v", name, mocks.names())
		}
	}
	if mocks.hasResource("dev-eu-1-bootstrap-root-application") {
		t.Fatal("root application should require an explicit GitOps repo URL")
	}

	argocd := mocks.resourceByName("dev-eu-1-bootstrap-argocd")
	if got := argocd.inputs["chart"].StringValue(); got != DefaultArgoCDChart {
		t.Fatalf("argocd chart = %q, want %s", got, DefaultArgoCDChart)
	}
	if _, ok := argocd.inputs["version"]; ok {
		t.Fatal("argocd version should not be set when using a pinned chart archive URL")
	}
	if got := argocd.inputs["namespace"].StringValue(); got != "platform-gitops" {
		t.Fatalf("argocd namespace = %q, want platform-gitops", got)
	}
	if _, ok := argocd.inputs["repositoryOpts"]; ok {
		t.Fatal("argocd repositoryOpts should not be set when using a pinned chart archive URL")
	}

	operator := mocks.resourceByName("dev-eu-1-bootstrap-pulumi-kubernetes-operator")
	if got := operator.inputs["chart"].StringValue(); got != DefaultPulumiOperatorChart {
		t.Fatalf("operator chart = %q, want %s", got, DefaultPulumiOperatorChart)
	}
	if got := operator.inputs["version"].StringValue(); got != DefaultPulumiOperatorChartVersion {
		t.Fatalf("operator version = %q, want %s", got, DefaultPulumiOperatorChartVersion)
	}
	if got := operator.inputs["namespace"].StringValue(); got != "platform-pulumi" {
		t.Fatalf("operator namespace = %q, want platform-pulumi", got)
	}

	operatorEnv := mocks.resourceByName("dev-eu-1-bootstrap-pulumi-kubernetes-operator-env")
	operatorEnvMetadata := operatorEnv.inputs["metadata"].ObjectValue()
	if got := operatorEnvMetadata["name"].StringValue(); got != DefaultPulumiOperatorEnvSecret {
		t.Fatalf("operator env secret name = %q, want %s", got, DefaultPulumiOperatorEnvSecret)
	}
	if got := operatorEnvMetadata["namespace"].StringValue(); got != "platform-pulumi" {
		t.Fatalf("operator env secret namespace = %q, want platform-pulumi", got)
	}
	operatorEnvStringData := operatorEnv.inputs["stringData"]
	if !operatorEnvStringData.IsSecret() {
		t.Fatal("operator env stringData is not marked secret")
	}
	if got := operatorEnvStringData.SecretValue().Element.ObjectValue()["PULUMI_CONFIG_PASSPHRASE"].StringValue(); got != "secret-passphrase" {
		t.Fatalf("operator env passphrase = %q, want test passphrase", got)
	}

	authDelegator := mocks.resourceByName("dev-eu-1-bootstrap-pulumi-kubernetes-operator-auth-delegator")
	authMetadata := authDelegator.inputs["metadata"].ObjectValue()
	if got := authMetadata["name"].StringValue(); got != "platform-pulumi:pulumi-kubernetes-operator:system:auth-delegator" {
		t.Fatalf("auth delegator binding name = %q, want operator auth-delegator binding", got)
	}
	roleRef := authDelegator.inputs["roleRef"].ObjectValue()
	if got := roleRef["name"].StringValue(); got != "system:auth-delegator" {
		t.Fatalf("auth delegator roleRef.name = %q, want system:auth-delegator", got)
	}
	subject := authDelegator.inputs["subjects"].ArrayValue()[0].ObjectValue()
	if got := subject["kind"].StringValue(); got != "ServiceAccount" {
		t.Fatalf("auth delegator subject kind = %q, want ServiceAccount", got)
	}
	if got := subject["name"].StringValue(); got != "pulumi-kubernetes-operator" {
		t.Fatalf("auth delegator subject name = %q, want pulumi-kubernetes-operator", got)
	}
	if got := subject["namespace"].StringValue(); got != "platform-pulumi" {
		t.Fatalf("auth delegator subject namespace = %q, want platform-pulumi", got)
	}
}

func TestNewBootstrapRegistersGitOpsRootApplicationWhenConfigured(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bootstrap, err := NewBootstrap(ctx, "dev-eu-1-bootstrap", Args{
			ClusterName:           "dev-eu-1",
			Kubeconfig:            pulumi.String("apiVersion: v1"),
			InstallArgoCD:         true,
			InstallPulumiOperator: true,
			GitOpsRoot: GitOpsRootSpec{
				RepoURL:        "https://github.com/example/platform.git",
				TargetRevision: "main",
				RootPath:       "gitops/root",
			},
		})
		if err != nil {
			return err
		}
		if bootstrap.RootApplication == nil {
			t.Fatal("RootApplication = nil, want Argo CD Application")
		}

		return nil
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	application := mocks.resourceByName("dev-eu-1-bootstrap-root-application")
	if application.name == "" {
		t.Fatalf("expected root application, got %#v", mocks.names())
	}
	if got := application.typ; got != "kubernetes:argoproj.io/v1alpha1:Application" {
		t.Fatalf("root application type = %q, want Argo CD Application", got)
	}
	if got := application.inputs["apiVersion"].StringValue(); got != "argoproj.io/v1alpha1" {
		t.Fatalf("root application apiVersion = %q, want argoproj.io/v1alpha1", got)
	}
	if got := application.inputs["kind"].StringValue(); got != "Application" {
		t.Fatalf("root application kind = %q, want Application", got)
	}

	metadata := application.inputs["metadata"].ObjectValue()
	if got := metadata["name"].StringValue(); got != "platform-root" {
		t.Fatalf("root application metadata.name = %q, want platform-root", got)
	}
	if got := metadata["namespace"].StringValue(); got != "platform-gitops" {
		t.Fatalf("root application metadata.namespace = %q, want platform-gitops", got)
	}

	spec := application.inputs["spec"].ObjectValue()
	if got := spec["project"].StringValue(); got != "default" {
		t.Fatalf("root application spec.project = %q, want default", got)
	}
	source := spec["source"].ObjectValue()
	if got := source["repoURL"].StringValue(); got != "https://github.com/example/platform.git" {
		t.Fatalf("root application repoURL = %q, want repo URL", got)
	}
	if got := source["targetRevision"].StringValue(); got != "main" {
		t.Fatalf("root application targetRevision = %q, want main", got)
	}
	if got := source["path"].StringValue(); got != "gitops/root" {
		t.Fatalf("root application path = %q, want gitops/root", got)
	}

	destination := spec["destination"].ObjectValue()
	if got := destination["server"].StringValue(); got != "https://kubernetes.default.svc" {
		t.Fatalf("root application destination.server = %q, want in-cluster API", got)
	}
	if got := destination["namespace"].StringValue(); got != "platform-gitops" {
		t.Fatalf("root application destination.namespace = %q, want platform-gitops", got)
	}

	syncPolicy := spec["syncPolicy"].ObjectValue()
	automated := syncPolicy["automated"].ObjectValue()
	if !automated["prune"].BoolValue() {
		t.Fatal("root application automated.prune = false, want true")
	}
	if !automated["selfHeal"].BoolValue() {
		t.Fatal("root application automated.selfHeal = false, want true")
	}
	if got := syncPolicy["syncOptions"].ArrayValue()[0].StringValue(); got != "CreateNamespace=true" {
		t.Fatalf("root application syncOptions[0] = %q, want CreateNamespace=true", got)
	}
}

func TestNewBootstrapRegistersCiliumRelease(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bootstrap, err := NewBootstrap(ctx, "dev-eu-1-bootstrap", Args{
			ClusterName:   "dev-eu-1",
			Kubeconfig:    pulumi.String("apiVersion: v1"),
			InstallCilium: true,
		})
		if err != nil {
			return err
		}
		if bootstrap.Cilium == nil {
			t.Fatal("Cilium release = nil, want release")
		}

		return nil
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	cilium := mocks.resourceByName("dev-eu-1-bootstrap-cilium")
	if cilium.name == "" {
		t.Fatalf("expected Cilium release, got %#v", mocks.names())
	}
	if got := cilium.inputs["chart"].StringValue(); got != DefaultCiliumChart {
		t.Fatalf("cilium chart = %q, want %s", got, DefaultCiliumChart)
	}
	if got := cilium.inputs["namespace"].StringValue(); got != "kube-system" {
		t.Fatalf("cilium namespace = %q, want kube-system", got)
	}
	values := cilium.inputs["values"].ObjectValue()
	ipam := values["ipam"].ObjectValue()
	if got := ipam["mode"].StringValue(); got != "kubernetes" {
		t.Fatalf("cilium ipam.mode = %q, want kubernetes", got)
	}
	if got := values["kubeProxyReplacement"].BoolValue(); got {
		t.Fatal("cilium kubeProxyReplacement = true, want false")
	}
	cgroup := values["cgroup"].ObjectValue()
	autoMount := cgroup["autoMount"].ObjectValue()
	if got := autoMount["enabled"].BoolValue(); got {
		t.Fatal("cilium cgroup.autoMount.enabled = true, want false")
	}
	if got := cgroup["hostRoot"].StringValue(); got != "/sys/fs/cgroup" {
		t.Fatalf("cilium cgroup.hostRoot = %q, want /sys/fs/cgroup", got)
	}
	operator := values["operator"].ObjectValue()
	if got := operator["replicas"].NumberValue(); got != 1 {
		t.Fatalf("cilium operator.replicas = %v, want 1", got)
	}
}

func TestNewBootstrapRegistersHCloudSecret(t *testing.T) {
	t.Parallel()

	mocks := &recordingMocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bootstrap, err := NewBootstrap(ctx, "dev-eu-1-bootstrap", Args{
			ClusterName: "dev-eu-1",
			Kubeconfig:  pulumi.String("apiVersion: v1"),
			HCloudToken: pulumi.String("secret-token"),
		})
		if err != nil {
			return err
		}
		if bootstrap.HCloudSecret == nil {
			t.Fatal("HCloudSecret = nil, want secret")
		}

		return nil
	}, pulumi.WithMocks("project", "stack", mocks))
	if err != nil {
		t.Fatalf("pulumi.RunErr() error = %v", err)
	}

	secret := mocks.resourceByName("dev-eu-1-bootstrap-hcloud-secret")
	if secret.name == "" {
		t.Fatalf("expected hcloud secret, got %#v", mocks.names())
	}
	metadata := secret.inputs["metadata"].ObjectValue()
	if got := metadata["name"].StringValue(); got != "hcloud" {
		t.Fatalf("secret metadata.name = %q, want hcloud", got)
	}
	if got := metadata["namespace"].StringValue(); got != "kube-system" {
		t.Fatalf("secret metadata.namespace = %q, want kube-system", got)
	}
	secretStringData := secret.inputs["stringData"]
	if !secretStringData.IsSecret() {
		t.Fatal("secret stringData is not marked secret")
	}
	stringData := secretStringData.SecretValue().Element.ObjectValue()
	if got := stringData["token"].StringValue(); got != "secret-token" {
		t.Fatalf("secret token = %q, want test token", got)
	}
}

func TestNewBootstrapRejectsInvalidArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args Args
		want string
	}{
		{
			name: "cluster",
			args: Args{Kubeconfig: pulumi.String("apiVersion: v1")},
			want: "cluster name",
		},
		{
			name: "kubeconfig",
			args: Args{ClusterName: "dev-eu-1"},
			want: "kubeconfig",
		},
		{
			name: "namespace",
			args: Args{
				ClusterName: "dev-eu-1",
				Kubeconfig:  pulumi.String("apiVersion: v1"),
				Namespaces:  []NamespaceSpec{{}},
			},
			want: "namespace[0].name",
		},
		{
			name: "duplicate namespace",
			args: Args{
				ClusterName: "dev-eu-1",
				Kubeconfig:  pulumi.String("apiVersion: v1"),
				Namespaces: []NamespaceSpec{
					{Name: "platform-system"},
					{Name: "platform-system"},
				},
			},
			want: "duplicated",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				_, err := NewBootstrap(ctx, "dev-eu-1-bootstrap", test.args)
				return err
			}, pulumi.WithMocks("project", "stack", &recordingMocks{}))
			if err == nil {
				t.Fatal("pulumi.RunErr() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("pulumi.RunErr() error = %q, want %q", err, test.want)
			}
		})
	}
}

type recordedResource struct {
	typ    string
	name   string
	inputs resource.PropertyMap
}

type recordingMocks struct {
	mu        sync.Mutex
	resources []recordedResource
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
		names = append(names, resource.name)
	}

	return names
}
