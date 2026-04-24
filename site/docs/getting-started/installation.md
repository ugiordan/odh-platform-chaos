# Installation

## Prerequisites

Before using Operator Chaos, ensure you have the following:

### Required for All Modes

- **Go 1.25+** - Check `go.mod` in the repository for the exact version required
- **controller-runtime v0.23+** - Required for SDK and Fuzz modes

!!! tip "No cluster? You can still use operator-chaos"
    Several commands work entirely offline: `validate`, `types`, `init`, and `preflight --local`. You can validate experiment YAML, scaffold new experiments, and lint knowledge models without any cluster access. See the [Offline vs Live Capabilities](../index.md#offline-vs-live-capabilities) table for the full list.

### Required for CLI and SDK Modes

- **Kubernetes/OpenShift cluster** - Required for running live experiments (`run`, `suite`, `preflight` without `--local`, `clean`)
  - Not required for fuzz testing (uses a fake client) or offline commands listed above
- **cluster-admin RBAC** - CLI experiments perform destructive operations including:
  - Pod deletion
  - RBAC revocation
  - Webhook mutation
  - NetworkPolicy creation

!!! warning "RBAC Requirements"
    CLI experiments require cluster-admin privileges because they perform intentionally destructive chaos operations. Never run experiments on production clusters without proper safeguards.

## Installation

Install the CLI using Go:

```bash
go install github.com/opendatahub-io/operator-chaos/cmd/operator-chaos@latest
```

This will install the `operator-chaos` binary to your `$GOPATH/bin` directory.

## Container Images

Pre-built container images are available at:

```
quay.io/opendatahub/operator-chaos:latest
```

Use these images for running the chaos controller in Kubernetes or for CI/CD pipelines.

## Verify Installation

Check that the installation was successful:

```bash
operator-chaos version
```

You should see the version information for the installed CLI.

## Next Steps

Choose your usage mode based on your testing needs:

- **[CLI Quickstart](../modes/cli.md)** - Run structured experiments against a live cluster
- **[SDK Quickstart](../modes/sdk.md)** - Inject API-level faults in your operator code
- **[Fuzz Quickstart](../modes/fuzz.md)** - Automated fault exploration during development
