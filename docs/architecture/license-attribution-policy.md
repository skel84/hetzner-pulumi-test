# License And Attribution Policy

## Project License Status

This repository does not yet declare a project-wide open-source license. Treat
first-party code as not licensed for external redistribution until a root
`LICENSE` file is intentionally added.

## Reference Repositories

The repositories under `examples/` are reference material only:

- `examples/terraform-hcloud-kubernetes`: MIT
- `examples/pulumi-hcloud-k8s`: MIT
- `examples/pulumi-talos-cluster`: Apache-2.0

Do not copy large chunks wholesale. Prefer clean-room first-party
implementations based on behavior, public APIs, validation rules, and tests.

## If Code Is Copied

When copying non-trivial code from a reference repository:

- preserve the upstream license notice required by that project;
- add a short source attribution comment near the copied code;
- document the copied file or function in this policy or a nearby package doc;
- keep the copied surface minimal and rewrite it when the first-party boundary is
  stable.

Apache-2.0 reference code may require preserving NOTICE content if present.
MIT reference code requires preserving the copyright and permission notice.

## Preferred Pattern

Use reference projects to identify the behavior the platform needs, then write
small typed Go packages and tests in this repository. Compatibility should be
proven through tests rather than by preserving upstream file structure.
