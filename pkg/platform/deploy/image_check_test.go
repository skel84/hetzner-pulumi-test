package deploy

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/apricote/hcloud-upload-image/hcloudimages"
	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func TestRequiredTalosImageSpecs(t *testing.T) {
	t.Parallel()

	env := validProgramEnvironment()
	env.NodePools.Workers = append(env.NodePools.Workers, config.NodePoolSpec{
		Name:         "arm",
		Type:         "cax11",
		Location:     "fsn1",
		Architecture: "arm64",
		Count:        1,
	})

	got := RequiredTalosImageSpecs(env)
	if len(got) != 2 {
		t.Fatalf("len(RequiredTalosImageSpecs()) = %d, want 2", len(got))
	}

	wantNames := []string{"talos-arm-v1.12.0", "talos-x86-v1.12.0"}
	if gotNames := RequiredTalosImages(env); !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("RequiredTalosImages() = %#v, want %#v", gotNames, wantNames)
	}

	amd64 := findImageSpec(t, got, "amd64")
	if amd64.Name != "talos-x86-v1.12.0" {
		t.Fatalf("amd64 name = %q, want talos-x86-v1.12.0", amd64.Name)
	}
	if amd64.Architecture != "amd64" {
		t.Fatalf("amd64 architecture = %q, want amd64", amd64.Architecture)
	}
	if !strings.Contains(amd64.URL, "/v1.12.0/hcloud-amd64.raw.xz") {
		t.Fatalf("amd64 URL = %q, want hcloud amd64 image URL", amd64.URL)
	}
	if amd64.Labels["image-name"] != "talos-x86-v1.12.0" {
		t.Fatalf("amd64 image-name label = %q, want talos-x86-v1.12.0", amd64.Labels["image-name"])
	}
}

func TestEnsureHCloudImagesUsesExistingManagedImage(t *testing.T) {
	spec := TalosImageSpec{
		Name:         "talos-x86-v1.12.0",
		Architecture: "amd64",
		TalosVersion: "v1.12.0",
		Labels:       talosImageLabels("talos-x86-v1.12.0", "v1.12.0", "amd64"),
	}
	finder := &fakeImageClient{
		images: []*hcloud.Image{
			{ID: 100, Created: time.Now().Add(-time.Hour)},
			{ID: 200, Created: time.Now()},
		},
	}
	uploader := &fakeImageUploader{}
	withImageClients(t, finder, uploader)

	result, err := EnsureHCloudImages(context.Background(), "secret-token", []TalosImageSpec{spec})
	if err != nil {
		t.Fatalf("EnsureHCloudImages() error = %v", err)
	}
	if result.Refs["amd64"] != "200" {
		t.Fatalf("amd64 ref = %q, want newest existing image ID 200", result.Refs["amd64"])
	}
	if !reflect.DeepEqual(result.Existing, []string{"talos-x86-v1.12.0"}) {
		t.Fatalf("Existing = %#v, want existing image", result.Existing)
	}
	if uploader.uploads != 0 {
		t.Fatalf("uploads = %d, want 0", uploader.uploads)
	}
	if !strings.Contains(finder.lastListOpts.LabelSelector, "image-name=talos-x86-v1.12.0") {
		t.Fatalf("label selector = %q, want image name", finder.lastListOpts.LabelSelector)
	}
}

func TestEnsureHCloudImagesUploadsMissingImage(t *testing.T) {
	spec := TalosImageSpec{
		Name:         "talos-x86-v1.12.0",
		Architecture: "amd64",
		TalosVersion: "v1.12.0",
		Location:     "nbg1",
		URL:          "https://factory.talos.dev/image/test/v1.12.0/hcloud-amd64.raw.xz",
		Labels:       talosImageLabels("talos-x86-v1.12.0", "v1.12.0", "amd64"),
	}
	finder := &fakeImageClient{}
	uploader := &fakeImageUploader{image: &hcloud.Image{ID: 300}}
	withImageClients(t, finder, uploader)

	result, err := EnsureHCloudImages(context.Background(), "secret-token", []TalosImageSpec{spec})
	if err != nil {
		t.Fatalf("EnsureHCloudImages() error = %v", err)
	}
	if result.Refs["amd64"] != "300" {
		t.Fatalf("amd64 ref = %q, want uploaded image ID 300", result.Refs["amd64"])
	}
	if !reflect.DeepEqual(result.Created, []string{"talos-x86-v1.12.0"}) {
		t.Fatalf("Created = %#v, want created image", result.Created)
	}
	if uploader.lastOptions.ImageURL.String() != spec.URL {
		t.Fatalf("upload URL = %q, want %q", uploader.lastOptions.ImageURL.String(), spec.URL)
	}
	if uploader.lastOptions.ImageCompression != hcloudimages.CompressionXZ {
		t.Fatalf("compression = %q, want xz", uploader.lastOptions.ImageCompression)
	}
	if uploader.lastOptions.Architecture != hcloud.ArchitectureX86 {
		t.Fatalf("architecture = %q, want x86", uploader.lastOptions.Architecture)
	}
	if uploader.lastOptions.Location.Name != "nbg1" {
		t.Fatalf("location = %q, want nbg1", uploader.lastOptions.Location.Name)
	}
	if finder.updatedLabels["image-name"] != "talos-x86-v1.12.0" {
		t.Fatalf("updated labels = %#v, want image-name label", finder.updatedLabels)
	}
}

func TestEnsureHCloudImagesCleansUpAfterUploadFailure(t *testing.T) {
	spec := TalosImageSpec{
		Name:         "talos-x86-v1.12.0",
		Architecture: "amd64",
		URL:          "https://factory.talos.dev/image/test/v1.12.0/hcloud-amd64.raw.xz",
		Labels:       talosImageLabels("talos-x86-v1.12.0", "v1.12.0", "amd64"),
	}
	finder := &fakeImageClient{}
	uploader := &fakeImageUploader{err: errors.New("upload failed")}
	withImageClients(t, finder, uploader)

	_, err := EnsureHCloudImages(context.Background(), "secret-token", []TalosImageSpec{spec})
	if err == nil {
		t.Fatal("EnsureHCloudImages() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "upload failed") {
		t.Fatalf("EnsureHCloudImages() error = %q, want upload error", err)
	}
	if uploader.cleanups != 1 {
		t.Fatalf("cleanups = %d, want 1", uploader.cleanups)
	}
}

func TestUploadManagedTalosImageRejectsNilUploadResult(t *testing.T) {
	t.Parallel()

	_, err := uploadManagedTalosImage(context.Background(), &fakeImageUploader{useNilImage: true}, TalosImageSpec{
		Name:         "talos-x86-v1.12.0",
		Architecture: "amd64",
		Location:     "nbg1",
		URL:          "https://factory.talos.dev/image/test/v1.12.0/hcloud-amd64.raw.xz",
	})
	if err == nil {
		t.Fatal("uploadManagedTalosImage() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "upload returned no image") {
		t.Fatalf("uploadManagedTalosImage() error = %q, want nil image context", err)
	}
}

func TestEnsureHCloudImagesRejectsMissingToken(t *testing.T) {
	t.Parallel()

	_, err := EnsureHCloudImages(context.Background(), "", []TalosImageSpec{{Name: "talos-x86-v1.12.0"}})
	if err == nil {
		t.Fatal("EnsureHCloudImages() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "HCLOUD_TOKEN") {
		t.Fatalf("EnsureHCloudImages() error = %q, want token context", err)
	}
}

func TestFindManagedTalosImageReturnsListError(t *testing.T) {
	t.Parallel()

	_, err := findManagedTalosImage(context.Background(), &fakeImageClient{listErr: errors.New("list failed")}, TalosImageSpec{Name: "talos-x86-v1.12.0"})
	if err == nil {
		t.Fatal("findManagedTalosImage() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "list failed") {
		t.Fatalf("findManagedTalosImage() error = %q, want list error", err)
	}
}

func TestUploadManagedTalosImageRejectsBadURL(t *testing.T) {
	t.Parallel()

	_, err := uploadManagedTalosImage(context.Background(), &fakeImageUploader{}, TalosImageSpec{
		Name: "talos-x86-v1.12.0",
		URL:  "://bad-url",
	})
	if err == nil {
		t.Fatal("uploadManagedTalosImage() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "parse Talos image URL") {
		t.Fatalf("uploadManagedTalosImage() error = %q, want URL context", err)
	}
}

func TestUploadManagedTalosImageRequiresHTTPSURL(t *testing.T) {
	t.Parallel()

	_, err := uploadManagedTalosImage(context.Background(), &fakeImageUploader{}, TalosImageSpec{
		Name: "talos-x86-v1.12.0",
		URL:  "http://factory.talos.dev/image/test/v1.12.0/hcloud-amd64.raw.xz",
	})
	if err == nil {
		t.Fatal("uploadManagedTalosImage() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "absolute https URL") {
		t.Fatalf("uploadManagedTalosImage() error = %q, want HTTPS context", err)
	}
}

func findImageSpec(t *testing.T, specs []TalosImageSpec, architecture string) TalosImageSpec {
	t.Helper()

	for _, spec := range specs {
		if spec.Architecture == architecture {
			return spec
		}
	}
	t.Fatalf("image spec for architecture %q was not found", architecture)
	return TalosImageSpec{}
}

func withImageClients(t *testing.T, finder hcloudImageClient, uploader imageUploader) {
	t.Helper()

	original := newHCloudImageClients
	newHCloudImageClients = func(string) (hcloudImageClient, imageUploader) {
		return finder, uploader
	}
	t.Cleanup(func() {
		newHCloudImageClients = original
	})
}

type fakeImageClient struct {
	images        []*hcloud.Image
	listErr       error
	updateErr     error
	lastListOpts  hcloud.ImageListOpts
	updatedLabels map[string]string
}

func (c *fakeImageClient) List(_ context.Context, opts hcloud.ImageListOpts) ([]*hcloud.Image, *hcloud.Response, error) {
	c.lastListOpts = opts
	return c.images, nil, c.listErr
}

func (c *fakeImageClient) Update(_ context.Context, image *hcloud.Image, opts hcloud.ImageUpdateOpts) (*hcloud.Image, *hcloud.Response, error) {
	c.updatedLabels = opts.Labels
	if opts.Description != nil {
		image.Description = *opts.Description
	}
	image.Labels = opts.Labels
	return image, nil, c.updateErr
}

type fakeImageUploader struct {
	image       *hcloud.Image
	err         error
	cleanupErr  error
	useNilImage bool
	uploads     int
	cleanups    int
	lastOptions hcloudimages.UploadOptions
}

func (u *fakeImageUploader) Upload(_ context.Context, opts hcloudimages.UploadOptions) (*hcloud.Image, error) {
	u.uploads++
	u.lastOptions = opts
	if u.err != nil {
		return nil, u.err
	}
	if u.useNilImage {
		return nil, nil
	}
	if u.image != nil {
		return u.image, nil
	}

	return &hcloud.Image{ID: 123}, nil
}

func (u *fakeImageUploader) CleanupTempResources(context.Context) error {
	u.cleanups++
	return u.cleanupErr
}
