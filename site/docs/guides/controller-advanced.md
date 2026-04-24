# Controller Advanced Guide

This guide covers advanced controller mode topics: additional experiment examples, detailed status fields, safety mechanisms, scheduled experiments, and GitOps integration. For getting started with controller mode, see [Controller Mode](../modes/controller.md).

## Additional Experiment Examples

### ConfigDrift Experiment

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: inferenceservice-config-drift
  namespace: operator-chaos-experiments
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller

  injection:
    type: ConfigDrift
    parameters:
      name: inferenceservice-config
      key: deploy
      value: "corrupted-config-data"
    ttl: "300s"

  hypothesis:
    description: >-
      When the inferenceservice-config ConfigMap is corrupted, the
      controller should detect and restore the correct configuration.
    recoveryTimeout: 180s

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

### RBACRevoke Experiment (High Danger)

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: rbac-revoke-test
  namespace: operator-chaos-experiments
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller

  injection:
    type: RBACRevoke
    parameters:
      bindingName: odh-model-controller-rolebinding-opendatahub
      bindingType: ClusterRoleBinding
    ttl: "300s"

  hypothesis:
    description: >-
      When the controller's RBAC binding is revoked, it should detect
      permission errors and recover when RBAC is restored.
    recoveryTimeout: 240s

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

!!! danger "High-Danger Injections"
    Injection types like `RBACRevoke` and `WebhookDisrupt` are high-danger. You must set `blastRadius.allowDangerous: true` or the controller will reject the experiment.

## Viewing Results

### Status Fields

The controller updates `.status` with experiment results:

```yaml
status:
  phase: Complete
  verdict: Resilient
  observedGeneration: 1
  message: "Experiment completed successfully"
  startTime: "2024-03-30T12:00:00Z"
  endTime: "2024-03-30T12:02:15Z"
  injectionStartedAt: "2024-03-30T12:00:05Z"

  steadyStatePre:
    passed: true
    checksRun: 1
    checksPassed: 1
    details:
      - check:
          type: conditionTrue
          apiVersion: apps/v1
          kind: Deployment
          name: odh-model-controller
          namespace: opendatahub
          conditionType: Available
        passed: true
        value: "True"
    timestamp: "2024-03-30T12:00:02Z"

  steadyStatePost:
    passed: true
    checksRun: 1
    checksPassed: 1
    details: [...]
    timestamp: "2024-03-30T12:02:06Z"

  injectionLog:
    - timestamp: "2024-03-30T12:00:05Z"
      type: PodKill
      target: opendatahub/odh-model-controller-5c7d8f9b-xz4k2
      action: deleted
      details:
        signal: SIGTERM

  evaluationResult:
    verdict: Resilient
    confidence: high
    recoveryTime: 115s
    reconcileCycles: 2
    deviations: []

  conditions:
    - type: SteadyStateEstablished
      status: "True"
      lastTransitionTime: "2024-03-30T12:00:02Z"
      reason: PreCheckPassed
      message: "Baseline steady-state established"
    - type: FaultInjected
      status: "True"
      lastTransitionTime: "2024-03-30T12:00:05Z"
      reason: InjectionSucceeded
      message: "Fault injected successfully"
    - type: RecoveryObserved
      status: "True"
      lastTransitionTime: "2024-03-30T12:02:05Z"
      reason: RecoveryComplete
      message: "Recovery timeout elapsed, all resources reconciled"
    - type: Complete
      status: "True"
      lastTransitionTime: "2024-03-30T12:02:15Z"
      reason: EvaluationComplete
      message: "Experiment complete, verdict: Resilient"
```

**Key status fields:**

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase (see lifecycle diagram) |
| `verdict` | string | Experiment verdict: `Resilient`, `Degraded`, `Failed`, `Inconclusive` |
| `message` | string | Human-readable status message |
| `startTime` | timestamp | When the experiment started |
| `endTime` | timestamp | When the experiment completed (phase Complete or Aborted) |
| `injectionStartedAt` | timestamp | When the fault was injected |
| `steadyStatePre` | object | Pre-injection check results |
| `steadyStatePost` | object | Post-recovery check results |
| `injectionLog` | array | Detailed log of injection actions |
| `evaluationResult` | object | Verdict, recovery time, reconcile cycles, deviations |
| `conditions` | array | Kubernetes-native status conditions |

### Query Experiments

```bash
# List all experiments
kubectl get chaosexperiments -A

# Filter by verdict
kubectl get chaosexperiments -A -o json | \
  jq '.items[] | select(.status.verdict == "Failed") | .metadata.name'

# Show experiments in progress
kubectl get chaosexperiments -A -o json | \
  jq '.items[] | select(.status.phase != "Complete" and .status.phase != "Aborted")'

# Get detailed results
kubectl get chaosexperiment my-experiment -o yaml
```

### Events

The controller emits events at each phase transition:

```bash
kubectl get events --field-selector involvedObject.kind=ChaosExperiment

# LAST SEEN   TYPE      REASON              OBJECT
# 2m          Normal    PhaseTransition     chaosexperiment/my-experiment   Phase: Pending -> SteadyStatePre
# 2m          Normal    PhaseTransition     chaosexperiment/my-experiment   Phase: SteadyStatePre -> Injecting
# 2m          Normal    FaultInjected       chaosexperiment/my-experiment   Deleted pod odh-model-controller-5c7d8f9b-xz4k2
# 30s         Normal    PhaseTransition     chaosexperiment/my-experiment   Phase: Injecting -> Observing
# 5s          Normal    PhaseTransition     chaosexperiment/my-experiment   Phase: Observing -> SteadyStatePost
# 3s          Normal    PhaseTransition     chaosexperiment/my-experiment   Phase: SteadyStatePost -> Evaluating
# 2s          Normal    VerdictRendered     chaosexperiment/my-experiment   Verdict: Resilient (recovery: 115s, cycles: 2)
# 1s          Normal    PhaseTransition     chaosexperiment/my-experiment   Phase: Evaluating -> Complete
```

## Safety Mechanisms

### Distributed Locking

The controller uses Kubernetes Leases to prevent concurrent experiments on the same operator:

1. Before injecting, controller acquires a lease for the target operator
2. Lease name: `chaos-lock-<operator-name>`
3. Lease namespace: `operator-chaos-system` (configurable via `--lock-namespace`)
4. If another experiment holds the lease, the controller requeues with backoff

**View active locks:**

```bash
kubectl get leases -n operator-chaos-system
# NAME                              HOLDER                          AGE
# chaos-lock-odh-model-controller   my-experiment                   45s
```

The lock is released when the experiment reaches `Complete` or `Aborted`.

### Finalizers

The controller adds a finalizer (`chaos.operatorchaos.io/cleanup`) during the `Injecting` phase. This ensures:

- If the CR is deleted mid-experiment, the controller reverts the fault before deleting
- If the controller crashes, the finalizer prevents orphaned faults

**Crash recovery**: If the controller crashes during an experiment, on restart it:

1. Resumes from the last recorded phase
2. Re-runs cleanup logic if phase is `Aborted`
3. Removes the finalizer on terminal phases

### TTL-Based Auto-Cleanup

Faults have a time-to-live (`injection.ttl`). Even if the controller crashes, the framework's TTL cleanup logic (running in the `Observer`) will eventually revert the fault.

### Blast Radius Limits

The controller enforces blast radius constraints before injection:

- **`maxPodsAffected`**: Maximum pods that can be affected
- **`allowedNamespaces`**: Injection restricted to these namespaces
- **`forbiddenResources`**: Resources that must not be touched
- **`allowDangerous`**: High-danger injections require explicit opt-in

Experiments that violate constraints are rejected with phase `Aborted` and message explaining the violation.

## Scheduled Experiments with CronJobs

Run experiments on a schedule using Kubernetes CronJobs:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nightly-chaos
  namespace: operator-chaos-experiments
spec:
  schedule: "0 2 * * *"  # 2 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: chaos-job-runner
          containers:
            - name: create-experiment
              image: bitnami/kubectl:latest
              command:
                - /bin/sh
                - -c
                - |
                  cat <<EOF | kubectl apply -f -
                  apiVersion: chaos.operatorchaos.io/v1alpha1
                  kind: ChaosExperiment
                  metadata:
                    generateName: nightly-podkill-
                    namespace: operator-chaos-experiments
                  spec:
                    target:
                      operator: odh-model-controller
                      component: odh-model-controller
                    injection:
                      type: PodKill
                      parameters:
                        labelSelector: control-plane=odh-model-controller
                    hypothesis:
                      description: "Nightly pod kill test"
                      recoveryTimeout: 120s
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
                  EOF
          restartPolicy: OnFailure
```

**Note**: The ServiceAccount needs RBAC to create ChaosExperiment CRs.

## GitOps Integration

Store experiments in Git and sync with Argo CD or Flux:

```yaml
# argocd-application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: chaos-experiments
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/my-org/chaos-experiments
    path: experiments/odh-model-controller
    targetRevision: main
  destination:
    server: https://kubernetes.default.svc
    namespace: operator-chaos-experiments
  syncPolicy:
    automated:
      prune: true
      selfHeal: false  # Don't auto-heal to preserve experiment history
```

## Emergency Stop

If experiments are stuck or the controller is misbehaving:

```bash
# Delete the controller deployment (stops new experiments)
kubectl delete deployment operator-chaos-controller -n operator-chaos-system

# Use the CLI to clean up faults manually
operator-chaos clean --namespace <namespace>
```

## Troubleshooting

### Experiment stuck in Pending

**Check controller logs:**

```bash
kubectl logs -n operator-chaos-system deployment/operator-chaos-controller
```

**Common causes:**

- Validation error (missing knowledge model, unknown injection type)
- Failed to acquire lock (another experiment is running on same operator)
- RBAC permissions missing

### Experiment stuck in Observing

The controller is waiting for the recovery timeout to elapse. Check:

```bash
kubectl get chaosexperiment my-experiment -o jsonpath='{.spec.hypothesis.recoveryTimeout}'
# 120s

# Check how long we've been observing
kubectl get chaosexperiment my-experiment -o jsonpath='{.status.injectionStartedAt}'
```

### Verdict is Inconclusive

The pre-injection steady-state check failed. Check:

```bash
kubectl get chaosexperiment my-experiment -o jsonpath='{.status.steadyStatePre}'
# {"passed":false,"checksRun":1,"checksPassed":0,"details":[...]}
```

Verify the target resource is healthy before running the experiment.

### Finalizer not removed

If an experiment is stuck deleting with a finalizer:

```bash
# Check phase
kubectl get chaosexperiment my-experiment -o jsonpath='{.status.phase}'

# If phase is Complete or Aborted, force-remove finalizer
kubectl patch chaosexperiment my-experiment -p '{"metadata":{"finalizers":[]}}' --type=merge
```

## Next Steps

- Learn about [Knowledge Models](knowledge-models.md) to define operator semantics
- See [Failure Modes](../failure-modes/index.md) for all available fault types
- Read [CI Integration Guide](ci-integration.md) for pipeline integration
