# End-to-End Testing Guide

> **Production files:** The knowledge model and experiment YAMLs referenced in this guide
> are shipped as production files in `knowledge/` and `experiments/`. You can run them
> directly instead of creating them manually.

This walkthrough uses **odh-model-controller** --- the operator that manages InferenceService lifecycle, model serving, and NIM accounts --- to demonstrate every injection type on a real cluster.

## Prerequisites

- OpenShift / Kubernetes cluster with OpenDataHub installed
- `odh-model-controller` running in the `opendatahub` namespace
- CLI built: `go build -o odh-chaos ./cmd/odh-chaos`
- `kubectl` / `oc` configured with cluster-admin access

## Step 1: Knowledge Model

The odh-model-controller knowledge model is at `knowledge/odh-model-controller.yaml`. It defines:

- 1 component with 4 managed resources (Deployment, ConfigMap, ServiceAccount, Lease)
- 7 webhooks (3 mutating, 4 validating)
- 3 finalizers
- Steady-state check: Deployment Available condition

## Step 2: Base Experiments (7 injection types)

Each experiment is a standalone YAML file in `experiments/odh-model-controller/`:

| File | Injection Type | Target | Danger |
|------|---------------|--------|--------|
| `pod-kill.yaml` | PodKill | controller pods | standard |
| `config-drift.yaml` | ConfigDrift | inferenceservice-config ConfigMap | high |
| `network-partition.yaml` | NetworkPartition | controller network | standard |
| `crd-mutation.yaml` | CRDMutation | InferenceService spec | standard |
| `finalizer-block.yaml` | FinalizerBlock | InferenceService resource | standard |
| `webhook-disrupt.yaml` | WebhookDisrupt | validating webhook failurePolicy | high |
| `rbac-revoke.yaml` | RBACRevoke | controller ClusterRoleBinding | high |

> **Note:** The `crd-mutation.yaml` and `finalizer-block.yaml` experiments contain a
> placeholder resource name (`test-isvc`). Replace it with the name of an actual
> InferenceService deployed in the target namespace before running.

## Step 3: Run the Suite

Execute all experiments, generate a report, and clean up:

```bash
# Validate all experiments first
odh-chaos suite experiments/odh-model-controller/ --dry-run

# Run the full suite
odh-chaos suite experiments/odh-model-controller/ \
  --report-dir reports/ \
  --timeout 10m

# Review results
odh-chaos report reports/

# Clean up any leftover chaos artifacts
odh-chaos clean --namespace opendatahub
```

## Expected Verdicts

| Experiment | Healthy Operator | Broken Reconciler |
|------------|-----------------|-------------------|
| PodKill | Resilient | Failed |
| ConfigDrift | Resilient | Failed |
| NetworkPartition | Resilient | Degraded / Failed |
| CRDMutation | Resilient | Failed |
| FinalizerBlock | Resilient | Degraded |
| WebhookDisrupt | Resilient | Failed |
| RBACRevoke | Resilient | Failed |

## Advanced Scenarios

Beyond the 7 base injection types, the following experiments test operator-specific failure modes:

| File | Injection | Target | Risk |
|------|-----------|--------|------|
| `webhook-cert-corrupt.yaml` | ConfigDrift | Webhook TLS cert Secret | high |
| `leader-lease-corrupt.yaml` | CRDMutation | Leader election Lease holderIdentity | high |
| `ingress-config-corruption.yaml` | ConfigDrift | ConfigMap ingress key | high |

These are in `experiments/odh-model-controller/` alongside the base experiments.

### kserve Experiments

A separate experiment suite for kserve is available in `experiments/kserve/`:

| File | Injection | Target |
|------|-----------|--------|
| `main-controller-kill.yaml` | PodKill | kserve-controller-manager |
| `llm-controller-isolation.yaml` | NetworkPartition | llmisvc-controller-manager |
| `isvc-config-corruption.yaml` | ConfigDrift | inferenceservice-config |
| `isvc-validator-disrupt.yaml` | WebhookDisrupt | InferenceService validating webhook |

Use knowledge model: `knowledge/kserve.yaml`
