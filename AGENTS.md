# Project Agent Instructions

This repository is building a European-first, Hetzner-first platform factory
using Pulumi, Talos, Yoke, Argo CD/YokeCD, and eventually `platformctl`.

## Ownership Boundaries

- Pulumi owns Hetzner infrastructure, Talos bootstrap lifecycle, and bootstrap
  outputs such as kubeconfig and talosconfig.
- Yoke owns typed Kubernetes package rendering.
- Argo CD/YokeCD owns steady-state Kubernetes reconciliation after bootstrap.
- `platformctl` is the product command surface and should stay thin until the
  underlying package boundaries stabilize.

## Reference Repositories

The projects under `examples/` are read-only references unless a user explicitly
asks for edits there. Do not vendor or copy large chunks from reference projects.
Extract behavior, validation rules, and tests into small first-party packages.

## Secrets And Live Infrastructure

- Do not read `.env` unless a command truly requires credentials.
- Never print, commit, or log the Hetzner token or derived secrets.
- Do not create, mutate, or destroy live Hetzner resources without explicit user
  confirmation for that live operation.
- Do not commit kubeconfig, talosconfig, Pulumi state, or local secret files.

## Development Rules

- Prefer small Go packages with typed structs over untyped maps.
- Write tests before implementation for new behavior.
- Run `go test ./...` before marking implementation tasks complete.
- Keep generated Kubernetes package output pure and testable.
- Do not introduce shell-driven lifecycle workarounds without an ADR.
