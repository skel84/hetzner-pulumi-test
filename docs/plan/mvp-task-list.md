# MVP Task List

## Status Legend

- `[ ]` not started
- `[~]` in progress
- `[x]` complete

## Phase 0: Repository Foundation

- [x] Create initial Go module for first-party code.
- [x] Add `cmd/platformctl` skeleton.
- [x] Add `pkg/platform/config` package.
- [x] Add `pkg/platform/validation` package.
- [x] Add baseline `go test ./...` workflow.
- [x] Add `.gitignore` for Pulumi state/config outputs, kubeconfig, talosconfig,
      and local secret files.
- [x] Add license and attribution policy for reference-derived code.
- [x] Add root `AGENTS.md` with project-specific ownership rules.

## Phase 1: Environment Config

- [x] Define `EnvironmentCatalog` Go type.
- [x] Define `ClusterSpec`.
- [x] Define `AccessSpec`.
- [x] Define `NodePoolSpec`.
- [x] Define `PackageProfile`.
- [x] Add YAML loader for `config/environments.yaml`.
- [x] Validate cluster names.
- [x] Validate Hetzner locations and server architecture hints.
- [x] Validate API exposure rules.
- [x] Validate control plane count.
- [x] Validate network CIDR shape.
- [x] Add example `config/environments.yaml`.
- [x] Add table-driven tests for valid and invalid environment configs.

## Phase 2: Pulumi Hetzner Talos Component

- [x] Create `pkg/pulumi/hetznertalos`.
- [x] Define public `ClusterArgs`.
- [x] Define public `Cluster` outputs.
- [x] Add Hetzner provider construction.
- [x] Create private network.
- [x] Create control plane subnet.
- [x] Create worker subnet.
- [x] Create load balancer subnet if needed.
- [x] Create control plane placement group.
- [x] Create firewalls from `AccessSpec`.
- [x] Implement current-IP firewall source resolution outside the component.
- [x] Select required Talos image architectures from node pools.
- [x] Implement Talos image upload or lookup.
- [x] Create control plane servers.
- [x] Create worker servers.
- [x] Attach servers to private network with stable private IPs.
- [x] Create kube API load balancer when enabled.
- [x] Register Pulumi outputs for endpoint and node inventory.
- [x] Add Pulumi mock tests for generated resource shape.

## Phase 3: Talos Lifecycle MVP

- [x] Define internal `TalosLifecycle` boundary.
- [x] Implement Pulumiverse-backed machine secrets generation.
- [x] Implement control plane machine config generation.
- [x] Implement worker machine config generation.
- [x] Disable default Talos CNI for external Cilium bootstrap.
- [x] Add Hetzner CCM bootstrap manifest support.
- [x] Apply machine config to initial control plane.
- [x] Bootstrap Talos on initial control plane.
- [x] Apply machine config to remaining control planes.
- [x] Apply machine config to workers.
- [x] Generate kubeconfig output.
- [x] Generate talosconfig output.
- [x] Add readiness checks for kube API and nodes.
- [x] Document MVP limitations for upgrades.

## Phase 4: Pulumi Kubernetes Bootstrap

- [x] Decide the first cluster baseline is Pulumi-owned bootstrap.
- [x] Remove YokeCD/Flight render path from the MVP code path.
- [x] Document bootstrap ownership and deferred YokeCD evaluation.
- [x] Add Pulumi Kubernetes provider dependency.
- [x] Add bootstrap Kubernetes component boundary.
- [x] Use generated kubeconfig to configure the Kubernetes provider.
- [x] Install Cilium through the Pulumi Kubernetes provider.
- [x] Create the Hetzner CCM `hcloud` secret through the Pulumi Kubernetes provider.
- [x] Create `platform-system` namespace.
- [x] Create `platform-gitops` namespace.
- [x] Create `platform-pulumi` namespace.
- [x] Add Pod Security labels to bootstrap namespaces.
- [x] Add baseline NetworkPolicies that do not block bootstrap.
- [x] Add Pulumi mock tests for bootstrap Kubernetes resources.

## Phase 5: GitOps And PKO Handoff

- [x] Decide MVP installs Argo CD through Pulumi.
- [x] Install Argo CD through the Pulumi Kubernetes provider.
- [x] Install Pulumi Kubernetes Operator.
- [x] Define the GitOps root handoff model.
- [x] Create the root Argo CD Application or app-of-apps entry.
- [x] Create a minimal PKO `Stack` handoff example.
- [x] Add `platformctl doctor` checks for Argo CD and PKO health.

## Phase 6: `platformctl`

- [ ] Implement `platformctl env list`.
- [x] Implement `platformctl up <env>`.
- [x] Implement `platformctl down <env>`.
- [x] Implement `platformctl kubeconfig <env>`.
- [x] Implement `platformctl talosconfig <env>`.
- [x] Implement `platformctl doctor`.
- [x] Add command tests for config loading and dispatch.
- [x] Add docs for local credentials and required tools.

## Phase 7: Verification

- [x] Run `go test ./...`.
- [ ] Run Pulumi preview against a dummy config.
- [x] Create a real `dev` cluster.
- [x] Verify Kubernetes nodes are Ready.
- [x] Verify Cilium health.
- [x] Verify Hetzner CCM health.
- [x] Verify Argo CD health.
- [x] Verify Pulumi Kubernetes Operator health.
- [x] Verify minimal PKO Stack sync.
- [x] Retrieve kubeconfig through `platformctl kubeconfig dev`.
- [x] Retrieve talosconfig through `platformctl talosconfig dev`.
- [ ] Destroy the cluster.
- [ ] Confirm no MVP-owned Hetzner resources remain.

## Phase 8: Post-MVP Candidates

- [ ] Add Talos upgrade flow.
- [ ] Add Kubernetes upgrade flow.
- [ ] Add autoscaler support.
- [ ] Add Longhorn or another storage profile.
- [ ] Add observability package through Argo CD Helm/Kustomize.
- [ ] Evaluate PKO-backed observability infrastructure.
- [ ] Evaluate YokeCD for `ServiceApp`, tenant bundles, or observability
      conventions.
- [ ] Evaluate ATC for product-level platform APIs.
- [ ] Evaluate `crd2pulumi` for important CRD surfaces.
- [ ] Evaluate direct Talos API implementation.
- [ ] Evaluate custom Pulumi provider only if the component library hits real
      lifecycle or state limits.

## First 10 Tasks

1. Create Go module and root project metadata.
2. Add `config/environments.yaml` example.
3. Implement config structs and YAML loader.
4. Add config validation tests.
5. Create `pkg/pulumi/hetznertalos` package skeleton.
6. Implement Hetzner network/subnet/firewall resources.
7. Implement nodepool server creation with Pulumi mocks.
8. Add Talos lifecycle interface and Pulumiverse-backed MVP implementation.
9. Add Pulumi Kubernetes bootstrap boundary.
10. Install Argo CD and PKO through Pulumi after kubeconfig readiness.
