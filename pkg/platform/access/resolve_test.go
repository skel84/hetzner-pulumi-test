package access

import (
	"strings"
	"testing"

	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
)

func TestResolveCurrentIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		access    config.AccessSpec
		currentIP string
		want      []string
	}{
		{
			name: "replaces current-ip with ipv4 host cidr",
			access: config.AccessSpec{
				Mode:         config.AccessModeRestrictedPublic,
				AllowedCIDRs: []string{"current-ip"},
			},
			currentIP: "203.0.113.10",
			want:      []string{"203.0.113.10/32"},
		},
		{
			name: "preserves explicit cidrs",
			access: config.AccessSpec{
				Mode:         config.AccessModeRestrictedPublic,
				AllowedCIDRs: []string{"198.51.100.0/24", "current-ip"},
			},
			currentIP: "2001:db8::1",
			want:      []string{"198.51.100.0/24", "2001:db8::1/128"},
		},
		{
			name: "leaves private access unchanged",
			access: config.AccessSpec{
				Mode: config.AccessModePrivate,
			},
			currentIP: "203.0.113.10",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ResolveCurrentIP(tt.access, tt.currentIP)
			if err != nil {
				t.Fatalf("ResolveCurrentIP() error = %v", err)
			}
			if len(got.AllowedCIDRs) != len(tt.want) {
				t.Fatalf("AllowedCIDRs = %#v, want %#v", got.AllowedCIDRs, tt.want)
			}
			for index := range tt.want {
				if got.AllowedCIDRs[index] != tt.want[index] {
					t.Fatalf("AllowedCIDRs = %#v, want %#v", got.AllowedCIDRs, tt.want)
				}
			}
		})
	}
}

func TestResolveCurrentIPRejectsInvalidCurrentIP(t *testing.T) {
	t.Parallel()

	_, err := ResolveCurrentIP(config.AccessSpec{
		Mode:         config.AccessModeRestrictedPublic,
		AllowedCIDRs: []string{"current-ip"},
	}, "not-an-ip")

	if err == nil {
		t.Fatal("ResolveCurrentIP() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "current IP") {
		t.Fatalf("ResolveCurrentIP() error = %q, want current IP context", err)
	}
}
