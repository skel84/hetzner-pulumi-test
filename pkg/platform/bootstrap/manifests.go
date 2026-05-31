package bootstrap

import (
	"encoding/json"
	"fmt"
)

const DefaultHetznerCCMManifestURL = "https://raw.githubusercontent.com/hetznercloud/hcloud-cloud-controller-manager/v1.26.0/deploy/ccm.yaml"

type ManifestProfile struct {
	HetznerCCM                     Manifest
	AllowSchedulingOnControlPlanes bool
}

type Manifest struct {
	Enabled bool
	URL     string
}

func DefaultManifestProfile() ManifestProfile {
	return ManifestProfile{
		HetznerCCM: Manifest{
			Enabled: true,
			URL:     DefaultHetznerCCMManifestURL,
		},
	}
}

func TalosConfigPatch(profile ManifestProfile) (string, error) {
	extraManifests := []string{}
	if profile.HetznerCCM.Enabled {
		extraManifests = append(extraManifests, profile.HetznerCCM.URL)
	}

	patch := map[string]any{
		"cluster": map[string]any{
			"allowSchedulingOnControlPlanes": profile.AllowSchedulingOnControlPlanes,
			"network": map[string]any{
				"cni": map[string]any{
					"name": "none",
				},
			},
			"extraManifests": extraManifests,
		},
	}

	encoded, err := json.Marshal(patch)
	if err != nil {
		return "", fmt.Errorf("render Talos bootstrap manifest patch: %w", err)
	}

	return string(encoded), nil
}
