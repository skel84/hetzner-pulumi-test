package hetznertalos

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

type Subnets struct {
	ControlPlane string
	Workers      string
	LoadBalancer string
}

func DerivedSubnets(networkCIDR string, includeLoadBalancer bool) (Subnets, error) {
	prefix, err := netip.ParsePrefix(networkCIDR)
	if err != nil {
		return Subnets{}, fmt.Errorf("derive subnets from network CIDR %q: %w", networkCIDR, err)
	}
	if !prefix.Addr().Is4() {
		return Subnets{}, fmt.Errorf("derive subnets from network CIDR %q: only IPv4 is supported", networkCIDR)
	}

	required := 2
	if includeLoadBalancer {
		required = 3
	}

	subnets, err := firstIPv4Subnets(prefix.Masked(), 24, required)
	if err != nil {
		return Subnets{}, err
	}

	plan := Subnets{
		ControlPlane: subnets[0],
		Workers:      subnets[1],
	}
	if includeLoadBalancer {
		plan.LoadBalancer = subnets[2]
	}

	return plan, nil
}

func RequiredArchitectures(args ClusterArgs) []string {
	architectures := map[string]struct{}{}
	for _, pool := range args.ControlPlanePools {
		if pool.Architecture != "" {
			architectures[pool.Architecture] = struct{}{}
		}
	}
	for _, pool := range args.WorkerPools {
		if pool.Architecture != "" {
			architectures[pool.Architecture] = struct{}{}
		}
	}

	result := make([]string, 0, len(architectures))
	for architecture := range architectures {
		result = append(result, architecture)
	}
	sort.Strings(result)

	return result
}

func TalosImageReferences(args ClusterArgs) map[string]string {
	images := map[string]string{}
	for _, architecture := range RequiredArchitectures(args) {
		images[architecture] = talosImageName(architecture, args.TalosVersion)
	}

	return images
}

func privateIP(subnetCIDR string, hostOffset int) (string, error) {
	prefix, err := netip.ParsePrefix(subnetCIDR)
	if err != nil {
		return "", fmt.Errorf("derive private IP from subnet %q: %w", subnetCIDR, err)
	}
	if !prefix.Addr().Is4() {
		return "", fmt.Errorf("derive private IP from subnet %q: only IPv4 is supported", subnetCIDR)
	}

	base := ipv4ToUint32(prefix.Masked().Addr())
	addr := uint32ToIPv4(base + uint32(hostOffset))
	if !prefix.Contains(addr) {
		return "", fmt.Errorf("host offset %d is outside subnet %q", hostOffset, subnetCIDR)
	}

	return addr.String(), nil
}

func needsKubeAPILoadBalancer(args ClusterArgs) bool {
	return totalCount(args.ControlPlanePools) > 1
}

func totalCount(pools []NodePoolSpec) int {
	count := 0
	for _, pool := range pools {
		count += pool.Count
	}

	return count
}

func talosImageName(architecture string, talosVersion string) string {
	switch architecture {
	case "amd64":
		return "talos-x86-" + talosVersion
	case "arm64":
		return "talos-arm-" + talosVersion
	default:
		return "talos-" + strings.ToLower(architecture) + "-" + talosVersion
	}
}

func firstIPv4Subnets(prefix netip.Prefix, subnetBits int, count int) ([]string, error) {
	if prefix.Bits() > subnetBits {
		return nil, fmt.Errorf("network CIDR %q is too small for /%d subnets", prefix, subnetBits)
	}

	available := 1 << (subnetBits - prefix.Bits())
	if available < count {
		return nil, fmt.Errorf("network CIDR %q has %d /%d subnets, need %d", prefix, available, subnetBits, count)
	}

	base := ipv4ToUint32(prefix.Addr())
	size := uint32(1) << (32 - subnetBits)
	subnets := make([]string, 0, count)
	for index := 0; index < count; index++ {
		addr := uint32ToIPv4(base + uint32(index)*size)
		subnets = append(subnets, netip.PrefixFrom(addr, subnetBits).String())
	}

	return subnets, nil
}

func ipv4ToUint32(addr netip.Addr) uint32 {
	bytes := addr.As4()
	return uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
}

func uint32ToIPv4(value uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{
		byte(value >> 24),
		byte(value >> 16),
		byte(value >> 8),
		byte(value),
	})
}
