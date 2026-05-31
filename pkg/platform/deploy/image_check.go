package deploy

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/apricote/hcloud-upload-image/hcloudimages"
	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
	"github.com/francesco/hetzner_pulumi/pkg/pulumi/hetznertalos"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

const (
	defaultTalosSchematicID = "ce4c980550dd2ab1b17bbf2b08801c7eb59418eafe8f279833297925d67c7515"
	defaultImageLocation    = "fsn1"
)

type ImageEnsurer func(context.Context, string, []TalosImageSpec) (ImageEnsureResult, error)

type TalosImageSpec struct {
	Name         string
	Architecture string
	TalosVersion string
	Location     string
	URL          string
	Labels       map[string]string
}

type ImageEnsureResult struct {
	Refs     map[string]string
	Existing []string
	Created  []string
}

type hcloudImageClient interface {
	List(context.Context, hcloud.ImageListOpts) ([]*hcloud.Image, *hcloud.Response, error)
	Update(context.Context, *hcloud.Image, hcloud.ImageUpdateOpts) (*hcloud.Image, *hcloud.Response, error)
}

type imageUploader interface {
	Upload(context.Context, hcloudimages.UploadOptions) (*hcloud.Image, error)
	CleanupTempResources(context.Context) error
}

var newHCloudImageClients = func(token string) (hcloudImageClient, imageUploader) {
	client := hcloud.NewClient(hcloud.WithToken(token))
	return &client.Image, hcloudimages.NewClient(client)
}

func RequiredTalosImages(env config.EnvironmentSpec) []string {
	specs := RequiredTalosImageSpecs(env)
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	sort.Strings(names)

	return names
}

func RequiredTalosImageSpecs(env config.EnvironmentSpec) []TalosImageSpec {
	args := hetznertalos.ClusterArgsFromEnvironment(env)
	refs := hetznertalos.TalosImageReferences(args)
	locations := firstLocationByArchitecture(args)

	architectures := make([]string, 0, len(refs))
	for architecture := range refs {
		architectures = append(architectures, architecture)
	}
	sort.Strings(architectures)

	specs := make([]TalosImageSpec, 0, len(architectures))
	for _, architecture := range architectures {
		name := refs[architecture]
		location := locations[architecture]
		if location == "" {
			location = defaultImageLocation
		}

		specs = append(specs, TalosImageSpec{
			Name:         name,
			Architecture: architecture,
			TalosVersion: args.TalosVersion,
			Location:     location,
			URL:          talosImageFactoryURL(args.TalosVersion, architecture),
			Labels:       talosImageLabels(name, args.TalosVersion, architecture),
		})
	}

	return specs
}

func EnsureHCloudImages(ctx context.Context, token string, specs []TalosImageSpec) (ImageEnsureResult, error) {
	if token == "" {
		return ImageEnsureResult{}, fmt.Errorf("HCLOUD_TOKEN is required to ensure Talos images")
	}

	finder, uploader := newHCloudImageClients(token)
	result := ImageEnsureResult{Refs: map[string]string{}}

	for _, spec := range specs {
		image, err := findManagedTalosImage(ctx, finder, spec)
		if err != nil {
			return ImageEnsureResult{}, err
		}
		if image != nil {
			result.Refs[spec.Architecture] = strconv.FormatInt(image.ID, 10)
			result.Existing = append(result.Existing, spec.Name)
			continue
		}

		created, err := uploadManagedTalosImage(ctx, uploader, spec)
		if err != nil {
			cleanupErr := uploader.CleanupTempResources(ctx)
			if cleanupErr != nil {
				return ImageEnsureResult{}, fmt.Errorf("%w; cleanup temporary image resources: %v", err, cleanupErr)
			}
			return ImageEnsureResult{}, err
		}

		updated, _, err := finder.Update(ctx, created, hcloud.ImageUpdateOpts{
			Description: &spec.Name,
			Labels:      spec.Labels,
		})
		if err != nil {
			return ImageEnsureResult{}, fmt.Errorf("label uploaded Talos image %q: %w", spec.Name, err)
		}
		result.Refs[spec.Architecture] = strconv.FormatInt(updated.ID, 10)
		result.Created = append(result.Created, spec.Name)
	}

	return result, nil
}

func findManagedTalosImage(ctx context.Context, client hcloudImageClient, spec TalosImageSpec) (*hcloud.Image, error) {
	images, _, err := client.List(ctx, hcloud.ImageListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: talosImageLabelSelector(spec),
			PerPage:       50,
		},
		Type:         []hcloud.ImageType{hcloud.ImageTypeSnapshot},
		Status:       []hcloud.ImageStatus{hcloud.ImageStatusAvailable},
		Architecture: []hcloud.Architecture{hcloudArchitecture(spec.Architecture)},
	})
	if err != nil {
		return nil, fmt.Errorf("check Hetzner image %q: %w", spec.Name, err)
	}
	if len(images) == 0 {
		return nil, nil
	}

	sort.Slice(images, func(i, j int) bool {
		return images[i].Created.After(images[j].Created)
	})

	return images[0], nil
}

func uploadManagedTalosImage(ctx context.Context, uploader imageUploader, spec TalosImageSpec) (*hcloud.Image, error) {
	imageURL, err := url.Parse(spec.URL)
	if err != nil {
		return nil, fmt.Errorf("parse Talos image URL %q: %w", spec.URL, err)
	}
	if imageURL.Scheme != "https" || imageURL.Host == "" {
		return nil, fmt.Errorf("Talos image URL %q must be an absolute https URL", spec.URL)
	}

	image, err := uploader.Upload(ctx, hcloudimages.UploadOptions{
		ImageURL:         imageURL,
		ImageCompression: hcloudimages.CompressionXZ,
		ImageFormat:      hcloudimages.FormatRaw,
		Architecture:     hcloudArchitecture(spec.Architecture),
		Description:      &spec.Name,
		Labels:           spec.Labels,
		Location:         &hcloud.Location{Name: spec.Location},
	})
	if err != nil {
		return nil, fmt.Errorf("upload Talos image %q: %w", spec.Name, err)
	}
	if image == nil {
		return nil, fmt.Errorf("upload Talos image %q: upload returned no image", spec.Name)
	}

	return image, nil
}

func firstLocationByArchitecture(args hetznertalos.ClusterArgs) map[string]string {
	locations := map[string]string{}
	for _, pool := range args.ControlPlanePools {
		if _, ok := locations[pool.Architecture]; !ok {
			locations[pool.Architecture] = pool.Location
		}
	}
	for _, pool := range args.WorkerPools {
		if _, ok := locations[pool.Architecture]; !ok {
			locations[pool.Architecture] = pool.Location
		}
	}

	return locations
}

func talosImageFactoryURL(talosVersion string, architecture string) string {
	return fmt.Sprintf("https://factory.talos.dev/image/%s/%s/hcloud-%s.raw.xz", defaultTalosSchematicID, talosVersion, talosFactoryArchitecture(architecture))
}

func talosFactoryArchitecture(architecture string) string {
	if architecture == "amd64" {
		return "amd64"
	}

	return architecture
}

func hcloudArchitecture(architecture string) hcloud.Architecture {
	if architecture == "arm64" {
		return hcloud.ArchitectureARM
	}

	return hcloud.ArchitectureX86
}

func talosImageLabels(name string, talosVersion string, architecture string) map[string]string {
	return map[string]string{
		"managed-by":   "platformctl",
		"kind":         "talos-image",
		"image-name":   name,
		"os":           "talos",
		"version":      talosVersion,
		"architecture": architecture,
	}
}

func talosImageLabelSelector(spec TalosImageSpec) string {
	parts := make([]string, 0, len(spec.Labels))
	for key, value := range spec.Labels {
		parts = append(parts, key+"="+value)
	}
	sort.Strings(parts)

	return strings.Join(parts, ",")
}
