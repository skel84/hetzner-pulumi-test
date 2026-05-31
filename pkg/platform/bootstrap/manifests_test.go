package bootstrap

import (
	"encoding/json"
	"testing"
)

func TestTalosConfigPatchIncludesHetznerCCM(t *testing.T) {
	t.Parallel()

	patch, err := TalosConfigPatch(DefaultManifestProfile())
	if err != nil {
		t.Fatalf("TalosConfigPatch() error = %v", err)
	}

	var decoded struct {
		Cluster struct {
			AllowSchedulingOnControlPlanes bool `json:"allowSchedulingOnControlPlanes"`
			Network                        struct {
				CNI struct {
					Name string `json:"name"`
				} `json:"cni"`
			} `json:"network"`
			ExtraManifests []string `json:"extraManifests"`
		} `json:"cluster"`
	}
	if err := json.Unmarshal([]byte(patch), &decoded); err != nil {
		t.Fatalf("unmarshal patch: %v\n%s", err, patch)
	}

	if decoded.Cluster.Network.CNI.Name != "none" {
		t.Fatalf("cluster.network.cni.name = %q, want none", decoded.Cluster.Network.CNI.Name)
	}
	if decoded.Cluster.AllowSchedulingOnControlPlanes {
		t.Fatal("cluster.allowSchedulingOnControlPlanes = true, want false by default")
	}

	assertContains(t, decoded.Cluster.ExtraManifests, DefaultHetznerCCMManifestURL)
}

func TestTalosConfigPatchAllowsControlPlaneScheduling(t *testing.T) {
	t.Parallel()

	profile := DefaultManifestProfile()
	profile.AllowSchedulingOnControlPlanes = true

	patch, err := TalosConfigPatch(profile)
	if err != nil {
		t.Fatalf("TalosConfigPatch() error = %v", err)
	}

	var decoded struct {
		Cluster struct {
			AllowSchedulingOnControlPlanes bool `json:"allowSchedulingOnControlPlanes"`
		} `json:"cluster"`
	}
	if err := json.Unmarshal([]byte(patch), &decoded); err != nil {
		t.Fatalf("unmarshal patch: %v\n%s", err, patch)
	}
	if !decoded.Cluster.AllowSchedulingOnControlPlanes {
		t.Fatal("cluster.allowSchedulingOnControlPlanes = false, want true")
	}
}

func TestTalosConfigPatchOmitsDisabledManifests(t *testing.T) {
	t.Parallel()

	patch, err := TalosConfigPatch(ManifestProfile{
		HetznerCCM: Manifest{
			Enabled: false,
			URL:     DefaultHetznerCCMManifestURL,
		},
	})
	if err != nil {
		t.Fatalf("TalosConfigPatch() error = %v", err)
	}

	var decoded struct {
		Cluster struct {
			ExtraManifests []string `json:"extraManifests"`
		} `json:"cluster"`
	}
	if err := json.Unmarshal([]byte(patch), &decoded); err != nil {
		t.Fatalf("unmarshal patch: %v\n%s", err, patch)
	}

	if len(decoded.Cluster.ExtraManifests) != 0 {
		t.Fatalf("extraManifests = %#v, want empty", decoded.Cluster.ExtraManifests)
	}
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()

	for _, value := range values {
		if value == want {
			return
		}
	}

	t.Fatalf("%#v does not contain %q", values, want)
}
