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

## Step 1: Create the Knowledge Model

Save this as `knowledge/odh-model-controller.yaml`:

```yaml
operator:
  name: odh-model-controller
  namespace: opendatahub
  repository: https://github.com/opendatahub-io/odh-model-controller

components:
  - name: odh-model-controller
    controller: DataScienceCluster
    managedResources:
      - apiVersion: apps/v1
        kind: Deployment
        name: odh-model-controller
        namespace: opendatahub
        labels:
          control-plane: odh-model-controller
        ownerRef: InferenceService
        expectedSpec:
          replicas: 1
      - apiVersion: v1
        kind: ConfigMap
        name: inferenceservice-config
        namespace: opendatahub
        ownerRef: InferenceService
      - apiVersion: v1
        kind: ServiceAccount
        name: odh-model-controller
        namespace: opendatahub
    dependencies:
      - kserve
    steadyState:
      checks:
        - type: conditionTrue
          apiVersion: apps/v1
          kind: Deployment
          name: odh-model-controller
          namespace: opendatahub
          conditionType: Available
      timeout: "60s"

recovery:
  reconcileTimeout: "300s"
  maxReconcileCycles: 10
```

## Step 2: Create Experiments

Create a directory `experiments/odh-model-controller/` with one file per injection type.

### 2a. PodKill --- terminate controller pods

```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: odh-model-controller-pod-kill
  labels:
    component: odh-model-controller
    severity: standard
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller
    resource: Deployment/odh-model-controller
  hypothesis:
    description: "odh-model-controller recovers from pod termination within 60s"
    recoveryTimeout: "60s"
  injection:
    type: PodKill
    parameters:
      labelSelector: "control-plane=odh-model-controller"
    count: 1
    ttl: "300s"
  observation:
    interval: "5s"
    duration: "120s"
    trackReconcileCycles: true
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: odh-model-controller
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

### 2b. ConfigDrift --- corrupt inferenceservice-config ConfigMap

```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: odh-model-controller-config-drift
  labels:
    component: odh-model-controller
    severity: standard
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller
    resource: ConfigMap/inferenceservice-config
  hypothesis:
    description: "Operator reconciles inferenceservice-config back to expected state within 120s"
    recoveryTimeout: "120s"
  injection:
    type: ConfigDrift
    parameters:
      name: "inferenceservice-config"
      key: "deploy"
      value: "{}"
    ttl: "300s"
  observation:
    interval: "5s"
    duration: "180s"
    trackReconcileCycles: true
  steadyState:
    checks:
      - type: resourceExists
        apiVersion: v1
        kind: ConfigMap
        name: inferenceservice-config
        namespace: opendatahub
    timeout: "30s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

### 2c. NetworkPartition --- block controller network traffic

```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: odh-model-controller-network-partition
  labels:
    component: odh-model-controller
    severity: standard
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller
  hypothesis:
    description: "Controller recovers after network partition is lifted within 120s"
    recoveryTimeout: "120s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: "control-plane=odh-model-controller"
    ttl: "60s"
  observation:
    interval: "5s"
    duration: "180s"
    trackReconcileCycles: true
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: odh-model-controller
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

### 2d. CRDMutation --- mutate an InferenceService spec field

```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: odh-model-controller-crd-mutation
  labels:
    component: odh-model-controller
    severity: standard
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller
  hypothesis:
    description: "Operator detects mutated InferenceService and reconciles within 120s"
    recoveryTimeout: "120s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "serving.kserve.io/v1beta1"
      kind: "InferenceService"
      name: "example-isvc"
      field: "spec.predictor.minReplicas"
      value: "999"
    ttl: "300s"
  observation:
    interval: "5s"
    duration: "180s"
    trackReconcileCycles: true
  steadyState:
    checks:
      - type: resourceExists
        apiVersion: serving.kserve.io/v1beta1
        kind: InferenceService
        name: example-isvc
        namespace: opendatahub
    timeout: "30s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

> **Note:** Replace `example-isvc` with the name of an actual InferenceService in your cluster.

### 2e. FinalizerBlock --- add a stuck finalizer to the controller Deployment

```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: odh-model-controller-finalizer-block
  labels:
    component: odh-model-controller
    severity: standard
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller
    resource: Deployment/odh-model-controller
  hypothesis:
    description: "Operator handles blocked deletion gracefully within 120s"
    recoveryTimeout: "120s"
  injection:
    type: FinalizerBlock
    parameters:
      kind: "Deployment"
      apiVersion: "apps/v1"
      name: "odh-model-controller"
      finalizer: "odh.inferenceservice.finalizers"
    ttl: "300s"
  observation:
    interval: "5s"
    duration: "180s"
    trackReconcileCycles: true
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: odh-model-controller
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

### 2f. WebhookDisrupt --- set validating webhook to Fail mode

```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: odh-model-controller-webhook-disrupt
  labels:
    component: odh-model-controller
    severity: high
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller
  hypothesis:
    description: "API availability recovers after webhook failurePolicy is restored within 120s"
    recoveryTimeout: "120s"
  injection:
    type: WebhookDisrupt
    parameters:
      webhookName: "validating.odh-model-controller.opendatahub.io"
    ttl: "60s"
    dangerLevel: high
  observation:
    interval: "5s"
    duration: "180s"
    trackReconcileCycles: true
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: odh-model-controller
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
    allowDangerous: true
```

### 2g. RBACRevoke --- remove subjects from the controller ClusterRoleBinding

```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: odh-model-controller-rbac-revoke
  labels:
    component: odh-model-controller
    severity: high
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller
  hypothesis:
    description: "Controller detects permission loss and recovers after RBAC is restored within 120s"
    recoveryTimeout: "120s"
  injection:
    type: RBACRevoke
    parameters:
      bindingName: "odh-model-controller-rolebinding-opendatahub"
      bindingType: "ClusterRoleBinding"
    ttl: "60s"
    dangerLevel: high
  observation:
    interval: "5s"
    duration: "180s"
    trackReconcileCycles: true
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: odh-model-controller
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
    allowDangerous: true
```

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
| `webhook-policy-weaken.yaml` | WebhookDisrupt | Validating webhook failurePolicy → Ignore | high |
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
