package config

const (
	AccessModePrivate          AccessMode = "private"
	AccessModeRestrictedPublic AccessMode = "restricted-public"
)

type AccessMode string

type EnvironmentCatalog struct {
	Environments map[string]EnvironmentSpec `yaml:"environments"`
}

type EnvironmentSpec struct {
	Provider  string         `yaml:"provider"`
	Cluster   ClusterSpec    `yaml:"cluster"`
	Network   NetworkSpec    `yaml:"network"`
	Access    AccessSpec     `yaml:"access"`
	NodePools NodePoolsSpec  `yaml:"nodePools"`
	Packages  PackageProfile `yaml:"packages"`
	GitOps    GitOpsSpec     `yaml:"gitops"`
}

type ClusterSpec struct {
	Name              string `yaml:"name"`
	Region            string `yaml:"region"`
	TalosVersion      string `yaml:"talosVersion"`
	KubernetesVersion string `yaml:"kubernetesVersion"`
}

type NetworkSpec struct {
	CIDR string `yaml:"cidr"`
}

type AccessSpec struct {
	Mode         AccessMode `yaml:"mode"`
	AllowedCIDRs []string   `yaml:"allowedCidrs"`
}

type NodePoolsSpec struct {
	ControlPlane []NodePoolSpec `yaml:"controlPlane"`
	Workers      []NodePoolSpec `yaml:"workers"`
}

type NodePoolSpec struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type"`
	Location     string `yaml:"location"`
	Architecture string `yaml:"architecture"`
	Count        int    `yaml:"count"`
}

type PackageProfile struct {
	ClusterBaseline    bool `yaml:"clusterBaseline"`
	GitOpsControlPlane bool `yaml:"gitopsControlPlane"`
	Secrets            bool `yaml:"secrets"`
	CertsDNS           bool `yaml:"certsDns"`
}

type GitOpsSpec struct {
	RepoURL        string `yaml:"repoUrl"`
	TargetRevision string `yaml:"targetRevision"`
	RootPath       string `yaml:"rootPath"`
}
