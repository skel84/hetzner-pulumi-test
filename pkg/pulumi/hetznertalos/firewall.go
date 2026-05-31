package hetznertalos

import (
	"github.com/pulumi/pulumi-hcloud/sdk/go/hcloud"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func controlPlaneFirewallRules(access AccessArgs) hcloud.FirewallRuleArray {
	if access.Mode == "private" {
		return hcloud.FirewallRuleArray{}
	}

	rules := hcloud.FirewallRuleArray{}
	for _, port := range []string{"6443", "50000", "50001"} {
		rules = append(rules, firewallRule(port, access.AllowedCIDRs))
	}

	return rules
}

func workerFirewallRules(access AccessArgs) hcloud.FirewallRuleArray {
	if access.Mode == "private" {
		return hcloud.FirewallRuleArray{}
	}

	return hcloud.FirewallRuleArray{
		firewallRule("50000", access.AllowedCIDRs),
	}
}

func firewallRule(port string, sourceCIDRs []string) *hcloud.FirewallRuleArgs {
	return &hcloud.FirewallRuleArgs{
		Direction: pulumi.String("in"),
		Protocol:  pulumi.String("tcp"),
		Port:      pulumi.String(port),
		SourceIps: pulumi.ToStringArray(sourceCIDRs),
	}
}
