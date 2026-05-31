# Talos Lifecycle MVP Limitations

The MVP Talos lifecycle is create-first and intentionally narrow.

## Supported In The MVP

- generate Talos machine secrets with Pulumiverse `pulumi-talos`;
- generate control-plane and worker machine configurations;
- apply machine config to the first control-plane node;
- bootstrap the cluster from the first control-plane node;
- apply machine config to remaining control-plane and worker nodes;
- retrieve kubeconfig and talosconfig outputs;
- run a Talos health check after bootstrap resources are wired.

## Not Supported Yet

- Talos OS upgrades;
- Kubernetes version upgrades;
- etcd member replacement workflows;
- node reset/recovery orchestration;
- cluster certificate rotation;
- worker pool rolling replacement;
- migrating from Pulumiverse resources to direct Talos API calls.

## Upgrade Policy For MVP

Do not treat changes to `talosVersion` or `kubernetesVersion` as a supported
upgrade path yet. The MVP may generate new desired configuration, but it does
not define the sequencing, health gates, rollback behavior, or data-safety
checks required for production upgrades.

Before upgrade support is added, write an ADR that defines:

- the owner of Talos and Kubernetes upgrade state;
- whether upgrades use Pulumiverse, direct Talos APIs, or `talosctl`;
- ordering for control-plane and worker nodes;
- etcd health and quorum checks;
- rollback and failed-node recovery behavior;
- how generated Talos config changes are diffed and audited.
