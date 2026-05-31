# Reference Reuse Map

## Purpose

The projects under `examples/` are reference material for the MVP. They should
be mined for proven ideas, edge cases, tests, and lifecycle lessons. They should
not be copied wholesale into the first-party codebase.

All three examples are open-source projects with permissive licenses:

- `examples/terraform-hcloud-kubernetes`: MIT
- `examples/pulumi-hcloud-k8s`: MIT
- `examples/pulumi-talos-cluster`: Apache-2.0

If code is copied, preserve attribution and license notices. Prefer clean-room
rewrites based on behavior and tests for first-party components.

## `terraform-hcloud-kubernetes`

### Use As Reference For

- mature Hetzner/Talos topology decisions
- image factory and architecture selection
- firewall source semantics
- private network and subnet layout
- kube API load balancer behavior
- node naming and labels
- Talos config patch layering
- Talos health check and upgrade sequencing
- edge cases around autoscaler-created nodes

### Do Not Copy

- large HCL variable surface
- Packer-driven image upload as the default lifecycle
- `local-exec` orchestration
- Terraform state marker patterns
- all bundled addons
- production upgrade machinery in MVP

### MVP Extraction

Extract decisions, not files:

- define a minimal `ClusterSpec`
- define a minimal `AccessSpec`
- define a minimal `NodePoolSpec`
- port validation rules into Go tests
- write the Pulumi implementation directly against provider APIs

## `pulumi-hcloud-k8s`

### Use As Reference For

- Pulumi Go project structure
- Hetzner component wrappers
- nodepool construction
- label helpers
- image upload with `pulumi-hcloud-upload-image`
- Pulumi unit test style
- chart/application bootstrap lessons

### Do Not Copy

- cookiecutter project surface
- experimental two-phase deployment behavior
- broad Kubernetes addon installation through Pulumi
- global upgrade queues
- app/chart management as a Pulumi responsibility

### MVP Extraction

Use it as the closest implementation reference for:

- first Pulumi component layout
- server/network/firewall resources
- image upload wrapper
- Pulumi mocks in tests

Then reduce the scope.

## `pulumi-talos-cluster`

### Use As Reference For

- separating Talos config generation from apply lifecycle
- Pulumi component-provider schema ideas
- Talos machine apply ordering
- etcd readiness thinking
- kubeconfig/talosconfig output shape
- migration concerns around Pulumiverse

### Do Not Copy

- provider generation system for MVP
- talosctl shell orchestration as the default baseline
- Linux-runner-only assumptions
- full multi-language SDK surface

### MVP Extraction

Use it to design the internal Talos boundary:

```go
type TalosLifecycle interface {
    Generate(ctx context.Context, spec ClusterSpec) (GeneratedCluster, error)
    Apply(ctx context.Context, generated GeneratedCluster) error
    Bootstrap(ctx context.Context, generated GeneratedCluster) error
    Kubeconfig(ctx context.Context, generated GeneratedCluster) (string, error)
    Talosconfig(ctx context.Context, generated GeneratedCluster) (string, error)
}
```

The first implementation can use Pulumiverse resources. The boundary should
make a later direct Talos API implementation possible.

## Cross-Reference Table

| Concern | Primary Reference | MVP Approach |
| --- | --- | --- |
| Hetzner network | `terraform-hcloud-kubernetes`, `pulumi-hcloud-k8s` | Pulumi component |
| Firewalls | `terraform-hcloud-kubernetes` | Go validation plus Pulumi HCloud resources |
| Nodepools | all three, mostly `pulumi-hcloud-k8s` | typed `NodePoolSpec` |
| Image upload | `terraform-hcloud-kubernetes`, `pulumi-hcloud-k8s` | start with Pulumi image upload wrapper |
| Talos config | `terraform-hcloud-kubernetes`, `pulumi-talos-cluster` | local interface around Pulumiverse |
| Talos apply/bootstrap | `pulumi-talos-cluster` | MVP create-only lifecycle |
| Upgrades | all three | post-MVP |
| CNI/bootstrap manifests | `terraform-hcloud-kubernetes`, `pulumi-hcloud-k8s` | minimal bootstrap only |
| Addons | `platform-infra` idea, not copied here | Argo CD Helm/Kustomize, or PKO-backed Pulumi stacks when stateful |
| Platform APIs | `platform-infra` idea | post-MVP PKO, YokeCD, or ATC exploration |

## Reference Principles

- Copy behavior only when the license and attribution are clear.
- Rewrite complicated lifecycle code in smaller typed components.
- Prefer tests that prove compatibility over line-by-line ports.
- When an example uses shell, ask whether a structured provider/API exists.
- Do not import work-specific code or naming from any private repository.
