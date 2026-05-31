# Local Deploy Runbook

This runbook is for the first short-lived Hetzner/Talos MVP deploy from a
developer machine.

## Required Local Tools

- Go matching `go.mod`.
- Pulumi CLI `v3.244.0` or newer.
- Hetzner `hcloud` CLI for optional read-only checks.
- A Hetzner Cloud API token in `.env` as `HCLOUD_TOKEN`.
- `PULUMI_CONFIG_PASSPHRASE` in `.env` for the local Pulumi backend.

Do not print `.env`, token values, or the Pulumi passphrase in logs.

## Talos Image Handling

`platformctl up` ensures required Talos images before running Pulumi apply. The
image labels and descriptions are derived from the Talos version and
architecture:

- `talos-x86-<talosVersion>` for `amd64`
- `talos-arm-<talosVersion>` for `arm64`

If a matching labeled snapshot already exists in the target Hetzner project,
`platformctl up` reuses it and passes the image ID to Pulumi. If it is missing,
`platformctl up` uploads the Talos Image Factory `hcloud-<arch>.raw.xz` artifact
through `hcloud-upload-image`, labels the resulting snapshot, and then passes
that image ID to Pulumi.

The upload path creates temporary Hetzner resources before the cluster apply
starts. The upload library cleans up its temporary server and SSH key on the
normal path, and `platformctl` also asks it to clean up temporary resources if
the upload fails. The reusable Talos snapshot remains in the project as an
intentional cache.

Reference: https://docs.siderolabs.com/talos/v1.12/platform-specific-installations/cloud-platforms/hetzner
Reference: https://github.com/apricote/hcloud-upload-image

## Safe Preview

```sh
set -a
[ -f .env ] && . ./.env
set +a

go run ./cmd/platformctl cluster preview \
  --env dev \
  --control-plane-count 1 \
  --worker-count 0
```

Preview is non-mutating. It resolves `current-ip` without printing the resolved
address. Preview does not upload missing Talos images.

## Apply

Only run apply after the preview is clean.

```sh
go run ./cmd/platformctl up dev \
  --control-plane-count 1 \
  --worker-count 0 \
  --yes
```

The `--yes` flag is required because this can create billable Hetzner resources,
including a reusable Talos snapshot when the image is not already present.
`platformctl up` streams Pulumi progress and errors live while keeping Pulumi
secret values redacted.

## Doctor

Run the built-in health checks after apply:

```sh
go run ./cmd/platformctl doctor dev
```

The doctor command reads the kubeconfig from Pulumi state into a temporary local
file and verifies:

- Kubernetes nodes are Ready.
- Cilium rolled out.
- Hetzner CCM rolled out.
- Argo CD application controller and server rolled out.
- Pulumi Kubernetes Operator rolled out.

## GitOps Handoff

The default `dev` config leaves `gitops.repoUrl` empty, so `platformctl up`
does not create a root Argo CD Application. This avoids pointing the live
cluster at a local-only workspace.

To enable the handoff, set `gitops.repoUrl` to a Git repository that Argo CD can
reach, keep `gitops.rootPath` set to `gitops/root`, and rerun `platformctl up`.
Pulumi will seed a `platform-root` Argo CD Application in `platform-gitops`.
Argo CD then owns the child resources under the root kustomization, including
the minimal PKO `Program` and `Stack` example.

Validate the Git-owned manifests against the live CRDs without creating them:

```sh
mkdir -p .pulumi/tmp
go run ./cmd/platformctl kubeconfig dev --out .pulumi/tmp/dev.kubeconfig
KUBECONFIG=.pulumi/tmp/dev.kubeconfig kubectl apply --dry-run=server -k gitops/root
```

The example PKO `Stack` intentionally uses `file:///tmp` inside the operator
workspace. It is only a handoff smoke test, not durable state storage.

## Retrieve Access Configs

Write kubeconfig and talosconfig to ignored local files when you need direct
tool access:

```sh
mkdir -p .pulumi/tmp
go run ./cmd/platformctl kubeconfig dev --out .pulumi/tmp/dev.kubeconfig
go run ./cmd/platformctl talosconfig dev --out .pulumi/tmp/dev.talosconfig
```

The `--out` path is written with `0600` permissions. Omitting `--out` prints the
requested config to stdout, which is useful for shell pipelines but should not be
used in shared logs.

## Destroy

Keep the first cluster short-lived. Destroy it from the same workspace after
verification.

```sh
go run ./cmd/platformctl down dev --yes
```

The `--yes` flag is required because this destroys live resources.
`platformctl down` also streams Pulumi progress and errors live.

After destroy, confirm no MVP-owned Hetzner resources remain in the project.
