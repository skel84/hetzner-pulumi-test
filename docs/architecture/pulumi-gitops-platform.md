# Pulumi And GitOps Platform Architecture

## Decision

Use Pulumi for infrastructure, Talos lifecycle, and the first Kubernetes
bootstrap layer. Use Argo CD for GitOps reconciliation. Use the Pulumi
Kubernetes Operator when a Git-owned package needs Pulumi's stateful provider
model.

YokeCD is deferred from the MVP. It remains a candidate for later stateless
manifest generation, but it is not on the critical path for the first working
cluster.

## Why

Pulumi already covers the surfaces the MVP needs:

- Hetzner infrastructure through the HCloud provider
- Talos bootstrap through the Pulumiverse Talos provider behind a local boundary
- Kubernetes bootstrap resources through the Pulumi Kubernetes provider
- GitOps-driven Pulumi programs through the Pulumi Kubernetes Operator
- typed CRD SDK generation through `crd2pulumi` when a CRD surface deserves it

Adding YokeCD now would add another renderer, artifact pipeline, and Argo CD
plugin before the platform has repeated package patterns that need it.

## Control Plane Split

```text
platformctl
  User-facing CLI and workflow orchestration.

Pulumi Automation API
  Executes the cluster stack and manages Pulumi state.

pkg/pulumi/hetznertalos
  Reusable Pulumi component for Hetzner Talos clusters.

Talos
  Node OS and Kubernetes bootstrap surface.

Pulumi Kubernetes provider
  Applies the first bootstrap Kubernetes resources after kubeconfig exists.

Argo CD
  Reconciles Git-owned Kubernetes resources after bootstrap.

Pulumi Kubernetes Operator
  Reconciles Git-owned Pulumi Stack resources when packages need Pulumi state.
```

## Ownership Rules

The same Kubernetes object must not be owned by Pulumi and Argo CD at the same
time.

Pulumi owns:

- Hetzner networks, firewalls, servers, images, and load balancers
- Talos machine secrets, machine config, apply, bootstrap, and outputs
- bootstrap namespaces required before Argo CD exists
- bootstrap Pod Security labels and baseline network policy
- Argo CD installation
- Pulumi Kubernetes Operator installation
- optional seed `platform-root` Argo CD Application when `gitops.repoUrl` is set

Argo CD owns:

- Git-sourced application sync
- Helm/Kustomize/plain manifest addons that do not need Pulumi state
- child Applications and app-of-apps below the Pulumi-owned root
- drift correction for Argo-owned Kubernetes resources

Pulumi Kubernetes Operator owns:

- Pulumi `Stack` custom resources synced by Argo CD
- stateful package bundles that need cloud APIs, imports, refresh, previews, or
  destroy semantics

## Package Guidance

Use Helm/Kustomize through Argo CD for commodity Kubernetes addons such as
Ingress controllers, External Secrets, cert-manager, and observability charts
unless the package needs coordinated cloud infrastructure.

Use PKO-backed Pulumi stacks for packages that cross the Kubernetes/cloud
boundary, for example object storage plus Kubernetes secrets plus chart values.

Use `crd2pulumi` when a CRD surface is important enough to deserve typed Pulumi
SDKs instead of untyped YAML.

Defer YokeCD until there is a repeated pure-Kubernetes composition problem that
is awkward in Helm/Kustomize and should not live in Pulumi state.

## MVP Flow

```text
config/environments.yaml
        |
        v
platformctl up dev
        |
        v
Pulumi Automation API
        |
        v
Hetzner + Talos cluster
        |
        v
kubeconfig output
        |
        v
Pulumi Kubernetes bootstrap
        |
        +--> platform namespaces and labels
        +--> baseline policies
        +--> Argo CD
        +--> Pulumi Kubernetes Operator
        |
        v
optional Pulumi-owned platform-root Application
        |
        v
Argo CD reconciles gitops/root
```

## References

- Pulumi Kubernetes provider: https://www.pulumi.com/registry/packages/kubernetes/
- Pulumi with Argo CD: https://www.pulumi.com/docs/iac/operations/continuous-delivery/argocd/
- Pulumi Kubernetes Operator: https://www.pulumi.com/docs/integrations/clouds/kubernetes/pulumi-kubernetes-operator/
- Pulumi Kubernetes integrations: https://www.pulumi.com/docs/integrations/clouds/kubernetes/
- `crd2pulumi`: https://www.pulumi.com/docs/integrations/clouds/kubernetes/crd2pulumi/
