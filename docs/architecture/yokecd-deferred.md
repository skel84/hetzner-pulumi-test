# YokeCD Deferred Evaluation

## Current Position

YokeCD is not part of the MVP.

The MVP uses Pulumi, the Pulumi Kubernetes provider, Argo CD, and the Pulumi
Kubernetes Operator. That stack is enough to create the cluster, bootstrap the
GitOps control plane, and reconcile programmable packages from Git.

## When YokeCD Might Fit

YokeCD may be useful later for pure Kubernetes package composition when all of
these are true:

- the package does not need cloud provider credentials or Pulumi state
- Helm values or Kustomize overlays have become too indirect or repetitive
- the desired abstraction is a typed product-level API
- Argo CD should own apply, prune, sync, and health
- the renderer should be sandboxed as a Wasm artifact

Good candidates include:

- `ServiceApp`
- tenant or namespace bundles
- observability conventions such as generated ServiceMonitors, dashboards, and
  alert rules
- Gateway API route bundles

## When Pulumi Or Argo Are Better

Use Pulumi or PKO when the package needs:

- external infrastructure
- provider credentials
- resource imports
- refresh or preview semantics
- explicit destroy behavior
- stateful dependency tracking across cloud and Kubernetes resources

Use plain Argo CD with Helm/Kustomize/manifests for commodity addons that are
already well-packaged.

## Revisit Trigger

Revisit YokeCD after the MVP has at least one repeated platform package whose
logic is too awkward for Helm/Kustomize but does not justify Pulumi state.
