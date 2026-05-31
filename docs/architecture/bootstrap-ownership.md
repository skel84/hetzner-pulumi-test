# Bootstrap Ownership

## Decision

The first cluster baseline is Pulumi-owned.

This includes the Kubernetes resources required before Argo CD can safely take
over steady-state reconciliation.

## Pulumi-Owned Bootstrap Resources

Pulumi should create these through the Pulumi Kubernetes provider once the Talos
cluster exposes a working kubeconfig:

- `platform-system` namespace
- `platform-gitops` namespace
- `platform-pulumi` namespace
- Pod Security labels on bootstrap namespaces
- namespace-local baseline NetworkPolicies where they do not block bootstrap
- Cilium Helm release
- `kube-system/hcloud` secret for Hetzner CCM
- Argo CD installation
- Pulumi Kubernetes Operator installation
- Pulumi Kubernetes Operator passphrase Secret sourced from local `.env`
- Pulumi Kubernetes Operator auth-delegator RBAC binding
- optional seed Argo CD `Application` named `platform-root`

These resources live in Pulumi state and should not also be rendered by Argo CD.

The seed `platform-root` Application is created only when
`gitops.repoUrl` is set for an environment. It points Argo CD at
`gitops.rootPath` in that repository. Keeping this one object Pulumi-owned gives
the first cluster a deterministic handoff point without requiring a separate
manual `kubectl apply`.

The default baseline may apply default-deny ingress to `platform-system`, but it
should not apply default-deny ingress to `platform-gitops` or `platform-pulumi`
until Argo CD and PKO traffic allow rules are defined.

## Argo-Owned Resources

After Argo CD is installed, Git should own:

- child applications and app-of-apps entries below the Pulumi-owned root
- commodity addon Helm releases
- static policies, dashboards, and application manifests
- PKO `Stack` custom resources

Argo-owned resources should not be duplicated in the cluster Pulumi stack.

## Handoff Rule

Only one reconciler owns a live Kubernetes object.

If an object needs to move from Pulumi to Argo CD later, that migration needs an
explicit task that removes the Pulumi resource from state or replaces the object
with a new Argo-owned resource.

## Deferred Package Renderers

YokeCD or other stateless renderers may be evaluated later, but they should
produce Argo-owned resources only. They must not render objects that the cluster
Pulumi stack already owns.
