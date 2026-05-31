# MVP Plan

## Purpose

Build a small but credible platform factory slice:

```text
platformctl up dev
```

That command should create a Hetzner Talos Kubernetes cluster, install the
minimal Pulumi-owned Kubernetes bootstrap layer, install Argo CD plus the Pulumi
Kubernetes Operator, and leave the operator with kubeconfig and talosconfig
outputs.

The MVP is designed for learning and product validation. It should be clean
enough for coding agents to extend without reverse-engineering a pile of HCL,
shell provisioners, generated YAML, and ownership exceptions.

## Product Thesis

The platform is not "Terraform rewritten in Pulumi" and not "Helm rewritten in
Go". The product is a repeatable platform factory for European, Hetzner-first
Kubernetes environments.

The narrow MVP thesis is:

- Pulumi is the infrastructure and bootstrap product SDK.
- Talos is the node operating system and cluster lifecycle substrate.
- Argo CD is the steady-state GitOps reconciler.
- The Pulumi Kubernetes Operator lets Argo CD reconcile Pulumi stacks when a
  package needs provider state.
- `platformctl` is the user-facing command surface.

YokeCD is deferred from the MVP. It remains a post-MVP option for stateless,
typed Kubernetes package rendering when repeated package patterns justify it.

## MVP Outcome

The MVP is complete when a developer can:

1. Define one environment in a typed config file.
2. Run one command to create a single Hetzner Talos cluster.
3. Retrieve kubeconfig and talosconfig.
4. Verify CNI and Hetzner cloud integration are healthy.
5. Install Pulumi-owned bootstrap namespaces and policies.
6. Install Argo CD plus the Pulumi Kubernetes Operator.
7. Handoff at least one Git-owned resource through Argo CD or PKO.
8. Destroy the cluster cleanly.

## Target Command Surface

The first CLI can be thin. It should exist to stabilize the product shape, not
to hide every implementation detail.

```text
platformctl env list
platformctl up dev
platformctl down dev
platformctl kubeconfig dev
platformctl talosconfig dev
platformctl doctor
```

Internally, `platformctl` calls Pulumi Automation API. The CLI should stay thin
until the underlying component boundaries are stable.

## MVP Scope

### In Scope

- one Hetzner Cloud project
- one cluster per environment
- Talos-based control plane and workers
- private Hetzner network
- restricted public or private Talos/Kubernetes API access
- kube API load balancer where needed
- Talos image factory/upload handling
- Pulumi-owned Cilium Helm bootstrap profile
- Hetzner CCM bootstrap profile
- Pulumi Kubernetes provider bootstrap after kubeconfig is available
- bootstrap namespaces, labels, and baseline policies
- Argo CD installation
- Pulumi Kubernetes Operator installation
- one GitOps/PKO handoff path
- local tests for config validation and Pulumi component shape

### Out Of Scope

- Yoke/YokeCD in the MVP critical path
- Cluster API Provider Hetzner
- Crossplane
- kro
- Longhorn
- autoscaler
- multi-cluster fleet management
- customer clusters
- full application platform APIs
- production upgrade orchestration
- a custom Pulumi provider
- replacing Argo CD
- replacing real Kubernetes operators with generated YAML

## Architecture Slice

```text
config/environments.yaml
        |
        v
platformctl
        |
        v
Pulumi Automation API
        |
        v
pkg/pulumi/hetznertalos
        |
        +--> Hetzner network, firewalls, servers, load balancer
        +--> Talos machine secrets, configs, apply, bootstrap
        +--> kubeconfig and talosconfig outputs
        |
        v
Pulumi Kubernetes provider
        |
        +--> bootstrap namespaces and policies
        +--> Argo CD
        +--> Pulumi Kubernetes Operator
        |
        v
Argo CD GitOps handoff
        |
        v
Helm/Kustomize/manifests and PKO Stack resources from Git
```

## Repository Shape

The MVP should grow toward this shape:

```text
cmd/platformctl/
  main.go

config/
  environments.yaml
  examples/

pkg/
  platform/config/
  platform/validation/
  pulumi/hetznertalos/
  pulumi/bootstrapk8s/

pulumi/
  stacks/
    cluster/

docs/
  architecture/
  plan/

examples/
  pulumi-hcloud-k8s/
  pulumi-talos-cluster/
  terraform-hcloud-kubernetes/
```

## Component Boundary

### `pkg/pulumi/hetznertalos`

Owns:

- Hetzner resources
- Talos machine lifecycle for initial cluster creation
- kubeconfig/talosconfig output
- cluster inventory outputs

Does not own:

- app platform APIs
- long-running GitOps reconciliation
- broad application package management

### Pulumi Kubernetes Bootstrap

Owns:

- bootstrap namespaces and labels
- baseline policies that must exist before GitOps
- Argo CD install
- Pulumi Kubernetes Operator install

Does not own:

- Argo-owned app manifests
- PKO Stack contents
- commodity addon drift after handoff

### Argo CD And PKO

Own:

- steady-state sync from Git
- drift correction for Argo-owned resources
- PKO Stack CRs and their Pulumi programs
- visibility into package health

Do not own:

- server creation
- Talos bootstrap
- Pulumi cluster-stack state

## Success Metrics

- A fresh cluster reaches Ready state from a clean environment config.
- A second `platformctl up dev` is idempotent.
- `platformctl down dev` destroys all MVP-owned Hetzner resources.
- `go test ./...` passes for config/component logic.
- Argo CD is healthy after bootstrap.
- PKO is installed and can observe/sync a Stack resource.
- No MVP Kubernetes resource is managed by more than one owner.
- No shell `local-exec` style workaround is introduced without a short ADR.

## MVP Risks

- Talos lifecycle through Pulumi/Pulumiverse may not cover upgrades cleanly.
- Hetzner image upload and image factory flows can add long-running steps.
- Bootstrap ordering can become flaky if cluster readiness is not explicit.
- Installing Kubernetes resources through Pulumi requires kubeconfig readiness.
- PKO adds another controller and state boundary.
- A custom provider may become tempting too early.

## Risk Responses

- Use Pulumiverse Talos behind a narrow internal interface for MVP speed.
- Keep direct Talos API or talosctl-backed implementation as a later swap.
- Treat upgrades as post-MVP.
- Make readiness checks explicit and testable.
- Keep Pulumi-owned bootstrap resources narrow.
- Use Argo CD Helm/Kustomize for commodity addons.
- Use PKO only when a package needs Pulumi state.
- Write an ADR before adding any custom provider or shell-driven lifecycle.
