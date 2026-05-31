package validation

import (
	"fmt"
	"net/netip"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
)

var clusterNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

var knownHetznerLocations = map[string]struct{}{
	"ash":  {},
	"fsn1": {},
	"hel1": {},
	"hil":  {},
	"nbg1": {},
	"sin":  {},
}

var knownArchitectures = map[string]struct{}{
	"amd64": {},
	"arm64": {},
}

func ValidateCatalog(catalog config.EnvironmentCatalog) error {
	if len(catalog.Environments) == 0 {
		return fmt.Errorf("environments: at least one environment is required")
	}

	names := make([]string, 0, len(catalog.Environments))
	for name := range catalog.Environments {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if err := ValidateEnvironment(catalog.Environments[name]); err != nil {
			return fmt.Errorf("environments.%s: %w", name, err)
		}
	}

	return nil
}

func ValidateEnvironment(env config.EnvironmentSpec) error {
	if err := validateClusterName(env.Cluster.Name); err != nil {
		return err
	}
	if err := validateNetworkCIDR(env.Network.CIDR); err != nil {
		return err
	}
	if err := validateAccess(env.Access); err != nil {
		return err
	}
	if err := validateNodePools(env.NodePools); err != nil {
		return err
	}
	if err := validateGitOps(env.GitOps); err != nil {
		return err
	}

	return nil
}

func validateClusterName(name string) error {
	if name == "" {
		return fmt.Errorf("cluster.name: required")
	}
	if len(name) > 63 {
		return fmt.Errorf("cluster.name: must be 63 characters or fewer")
	}
	if !clusterNamePattern.MatchString(name) {
		return fmt.Errorf("cluster.name: must contain only lowercase letters, numbers, and hyphens")
	}

	return nil
}

func validateNetworkCIDR(cidr string) error {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("network.cidr: invalid CIDR %q: %w", cidr, err)
	}
	if !prefix.Addr().Is4() {
		return fmt.Errorf("network.cidr: must be an IPv4 CIDR")
	}
	if !prefix.Addr().IsPrivate() {
		return fmt.Errorf("network.cidr: must use private IPv4 address space")
	}
	if prefix.Masked() != prefix {
		return fmt.Errorf("network.cidr: must use a network address, not a host address")
	}

	return nil
}

func validateAccess(access config.AccessSpec) error {
	switch access.Mode {
	case config.AccessModePrivate:
		if len(access.AllowedCIDRs) > 0 {
			return fmt.Errorf("access.allowedCidrs: must be empty when access.mode is private")
		}
	case config.AccessModeRestrictedPublic:
		if len(access.AllowedCIDRs) == 0 {
			return fmt.Errorf("access.allowedCidrs: at least one CIDR or current-ip is required")
		}
		for _, allowedCIDR := range access.AllowedCIDRs {
			if err := validateAllowedCIDR(allowedCIDR); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("access.mode: must be %q or %q", config.AccessModePrivate, config.AccessModeRestrictedPublic)
	}

	return nil
}

func validateAllowedCIDR(cidr string) error {
	if cidr == "current-ip" {
		return nil
	}

	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("access.allowedCidrs: invalid CIDR %q: %w", cidr, err)
	}
	if prefix.Masked() != prefix {
		return fmt.Errorf("access.allowedCidrs: %q must use a network address, not a host address", cidr)
	}

	return nil
}

func validateNodePools(nodePools config.NodePoolsSpec) error {
	if len(nodePools.ControlPlane) == 0 {
		return fmt.Errorf("nodePools.controlPlane: at least one pool is required")
	}

	totalControlPlaneCount := 0
	for index, pool := range nodePools.ControlPlane {
		if err := validateNodePool(fmt.Sprintf("nodePools.controlPlane[%d]", index), pool); err != nil {
			return err
		}
		totalControlPlaneCount += pool.Count
	}
	if totalControlPlaneCount%2 == 0 {
		return fmt.Errorf("nodePools.controlPlane: total count must be odd")
	}

	for index, pool := range nodePools.Workers {
		if err := validateNodePool(fmt.Sprintf("nodePools.workers[%d]", index), pool); err != nil {
			return err
		}
	}

	return nil
}

func validateNodePool(path string, pool config.NodePoolSpec) error {
	if pool.Name == "" {
		return fmt.Errorf("%s.name: required", path)
	}
	if pool.Type == "" {
		return fmt.Errorf("%s.type: required", path)
	}
	if pool.Location == "" {
		return fmt.Errorf("%s.location: required", path)
	}
	if _, ok := knownHetznerLocations[pool.Location]; !ok {
		return fmt.Errorf("%s.location: unknown Hetzner location %q", path, pool.Location)
	}
	if err := validateArchitecture(path, pool); err != nil {
		return err
	}
	if pool.Count < 1 {
		return fmt.Errorf("%s.count: must be at least 1", path)
	}

	return nil
}

func validateArchitecture(path string, pool config.NodePoolSpec) error {
	if pool.Architecture == "" {
		return fmt.Errorf("%s.architecture: required", path)
	}
	if _, ok := knownArchitectures[pool.Architecture]; !ok {
		return fmt.Errorf("%s.architecture: must be amd64 or arm64", path)
	}

	expected, ok := expectedArchitecture(pool.Type)
	if !ok {
		return nil
	}
	if pool.Architecture != expected {
		return fmt.Errorf("%s.architecture: server type %q expects %s", path, pool.Type, expected)
	}

	return nil
}

func expectedArchitecture(serverType string) (string, bool) {
	normalized := strings.ToLower(serverType)
	switch {
	case strings.HasPrefix(normalized, "cax"):
		return "arm64", true
	case strings.HasPrefix(normalized, "cx"), strings.HasPrefix(normalized, "cpx"), strings.HasPrefix(normalized, "ccx"):
		return "amd64", true
	default:
		return "", false
	}
}

func validateGitOps(gitops config.GitOpsSpec) error {
	repoURL := strings.TrimSpace(gitops.RepoURL)
	if repoURL == "" {
		return nil
	}
	if repoURL != gitops.RepoURL {
		return fmt.Errorf("gitops.repoUrl: must not include leading or trailing whitespace")
	}
	if !strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "ssh://") && !strings.HasPrefix(repoURL, "git@") {
		return fmt.Errorf("gitops.repoUrl: must use https://, ssh://, or git@ syntax")
	}

	if strings.TrimSpace(gitops.TargetRevision) != gitops.TargetRevision {
		return fmt.Errorf("gitops.targetRevision: must not include leading or trailing whitespace")
	}

	rootPath := strings.TrimSpace(gitops.RootPath)
	if rootPath == "" {
		return nil
	}
	if rootPath != gitops.RootPath {
		return fmt.Errorf("gitops.rootPath: must not include leading or trailing whitespace")
	}
	if strings.Contains(rootPath, "\\") {
		return fmt.Errorf("gitops.rootPath: must use forward slashes")
	}
	if strings.HasPrefix(rootPath, "/") {
		return fmt.Errorf("gitops.rootPath: must be relative")
	}
	cleanRootPath := path.Clean(rootPath)
	for _, segment := range strings.Split(cleanRootPath, "/") {
		if segment == ".." {
			return fmt.Errorf("gitops.rootPath: must not contain parent traversal")
		}
	}

	return nil
}
