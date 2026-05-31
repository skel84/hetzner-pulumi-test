package access

import (
	"fmt"
	"net/netip"

	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
)

const CurrentIPPlaceholder = "current-ip"

func ResolveCurrentIP(access config.AccessSpec, currentIP string) (config.AccessSpec, error) {
	if len(access.AllowedCIDRs) == 0 {
		return access, nil
	}

	resolved := config.AccessSpec{
		Mode:         access.Mode,
		AllowedCIDRs: make([]string, 0, len(access.AllowedCIDRs)),
	}
	for _, allowedCIDR := range access.AllowedCIDRs {
		if allowedCIDR != CurrentIPPlaceholder {
			resolved.AllowedCIDRs = append(resolved.AllowedCIDRs, allowedCIDR)
			continue
		}

		prefix, err := currentIPPrefix(currentIP)
		if err != nil {
			return config.AccessSpec{}, err
		}
		resolved.AllowedCIDRs = append(resolved.AllowedCIDRs, prefix)
	}

	return resolved, nil
}

func currentIPPrefix(value string) (string, error) {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String(), nil
	}

	addr, err := netip.ParseAddr(value)
	if err != nil {
		return "", fmt.Errorf("current IP %q is not a valid IP address or CIDR: %w", value, err)
	}
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32).String(), nil
	}

	return netip.PrefixFrom(addr, 128).String(), nil
}
