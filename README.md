# Hetzner Pulumi Platform

Working notes and planning docs for a European-first, Hetzner-first platform
factory built around Talos, Pulumi, Argo CD, and GitOps.

The current repository is in planning/bootstrap mode. The downloaded reference
projects live under `examples/` and are treated as source material, not as code
to vendor wholesale.

Start here:

- [MVP plan](docs/plan/mvp-plan.md)
- [MVP task list](docs/plan/mvp-task-list.md)
- [Pulumi and GitOps architecture](docs/architecture/pulumi-gitops-platform.md)
- [Bootstrap ownership](docs/architecture/bootstrap-ownership.md)
- [YokeCD deferred evaluation](docs/architecture/yokecd-deferred.md)
- [Reference project reuse map](docs/architecture/reference-reuse-map.md)
- [Local deploy runbook](docs/operations/local-deploy-runbook.md)

Current direction:

- Pulumi creates Hetzner infrastructure and Talos clusters.
- Pulumi Kubernetes provider owns the first bootstrap Kubernetes layer.
- Argo CD reconciles Git-owned Kubernetes resources.
- Pulumi Kubernetes Operator can reconcile programmable Pulumi stacks from Git.
- `platformctl` provides the eventual product command surface.

The first deliverable is not a full platform. It is one reproducible Hetzner
Talos cluster that reaches a GitOps/Pulumi handoff with clear tests and docs.
