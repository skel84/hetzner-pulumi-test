package hetznertalos

import (
	"reflect"
	"testing"
)

func TestDerivedSubnets(t *testing.T) {
	t.Parallel()

	subnets, err := DerivedSubnets("10.40.0.0/16", true)
	if err != nil {
		t.Fatalf("DerivedSubnets() error = %v", err)
	}

	want := Subnets{
		ControlPlane: "10.40.0.0/24",
		Workers:      "10.40.1.0/24",
		LoadBalancer: "10.40.2.0/24",
	}
	if subnets != want {
		t.Fatalf("DerivedSubnets() = %#v, want %#v", subnets, want)
	}
}

func TestRequiredArchitectures(t *testing.T) {
	t.Parallel()

	got := RequiredArchitectures(ClusterArgs{
		ControlPlanePools: []NodePoolSpec{
			{Architecture: "amd64"},
			{Architecture: "arm64"},
		},
		WorkerPools: []NodePoolSpec{
			{Architecture: "amd64"},
		},
	})

	want := []string{"amd64", "arm64"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RequiredArchitectures() = %#v, want %#v", got, want)
	}
}

func TestTalosImageReferences(t *testing.T) {
	t.Parallel()

	got := TalosImageReferences(ClusterArgs{
		TalosVersion: "v1.12.0",
		ControlPlanePools: []NodePoolSpec{
			{Architecture: "amd64"},
			{Architecture: "arm64"},
		},
	})

	want := map[string]string{
		"amd64": "talos-x86-v1.12.0",
		"arm64": "talos-arm-v1.12.0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TalosImageReferences() = %#v, want %#v", got, want)
	}
}
