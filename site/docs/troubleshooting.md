# Troubleshooting

Common issues and solutions when running chaos experiments.

## Experiment Fails to Load

**Symptom**: `Error: failed to load experiment: ...`

**Causes:**

- YAML syntax errors (indentation, missing quotes)
- Invalid injection type name
- Missing required parameters

**Fix:**

```bash
# Validate YAML syntax
yamllint my-experiment.yaml

# Dry-run validates structure without executing
operator-chaos run my-experiment.yaml --dry-run --verbose
```

## Steady-State Check Fails Before Injection

**Symptom**: `Error: steady-state check failed before injection`

The target component isn't healthy before the experiment starts. The framework won't inject faults into an already-broken system.

**Fix:**

```bash
# Verify the component is healthy
kubectl get deployment -n opendatahub
kubectl describe deployment my-controller -n opendatahub

# Check that the knowledge model matches your cluster
cat knowledge/my-operator.yaml

# Increase steady-state timeout if the component is slow to report ready
# In your experiment YAML:
#   steadyState:
#     timeout: "60s"  # increase from default 30s
```

## Blast Radius Violation

**Symptom**: `Error: blast radius check failed: ...`

**Common causes:**

- Injection would affect more pods than `maxPodsAffected`
- Target namespace not listed in `allowedNamespaces`
- High-danger injection (RBACRevoke, WebhookDisrupt, CRDMutation on Routes) without `allowDangerous: true`

**Fix:**

```yaml
blastRadius:
  maxPodsAffected: 5
  allowedNamespaces:
    - opendatahub
    - test-namespace
  allowDangerous: true  # required for high-danger injections
```

## Permission Denied

**Symptom**: `Error: ... is forbidden: User "..." cannot ...`

The CLI or controller ServiceAccount lacks RBAC permissions for the injection type.

**Fix:**

```bash
# Check your permissions
kubectl auth can-i delete pods -n opendatahub
kubectl auth can-i update configmaps -n opendatahub

# For controller mode, check the ServiceAccount
kubectl get clusterrole operator-chaos-role -o yaml
kubectl get clusterrolebinding operator-chaos-binding -o yaml
```

## Cleanup Doesn't Complete

**Symptom**: Resources still have chaos annotations/labels after experiment completes or is interrupted.

**Fix:**

```bash
# Automated cleanup scans for orphaned chaos artifacts
operator-chaos clean --namespace opendatahub

# Check for resources with chaos metadata
kubectl get all -n opendatahub \
  -l "chaos.operatorchaos.io/injected=true"

# Manual annotation removal (last resort)
kubectl annotate deployment my-controller \
  chaos.operatorchaos.io/rollback-data- \
  -n opendatahub
```

## Controller Mode: Experiment Stuck in Pending

**Common causes:**

- Missing knowledge model for the target operator
- Another experiment holds the distributed lock on the same operator
- Controller RBAC missing

**Fix:**

```bash
# Check controller logs
kubectl logs -n operator-chaos-system deployment/operator-chaos-controller

# Check for active locks
kubectl get leases -n operator-chaos-system
```

See the [Controller Advanced Guide](guides/controller-advanced.md) for more controller-specific troubleshooting.

## Controller Mode: Experiment Stuck in Observing

The controller is waiting for the recovery timeout to elapse. This is normal behavior.

```bash
# Check the configured timeout
kubectl get chaosexperiment my-experiment -o jsonpath='{.spec.hypothesis.recoveryTimeout}'

# Check when injection started
kubectl get chaosexperiment my-experiment -o jsonpath='{.status.injectionStartedAt}'
```

## Knowledge Model Mismatch

**Symptom**: Preflight passes but experiments fail because resource names, namespaces, or labels don't match what's actually on the cluster.

**Fix:**

```bash
# Run preflight to validate knowledge model against cluster
operator-chaos preflight --knowledge knowledge/my-operator.yaml

# Compare expected vs actual
kubectl get deployment -n opendatahub -o name
kubectl get configmap -n opendatahub -o name
```

Knowledge models have environment-specific overlays in `knowledge/rhoai/` for RHOAI deployments. Make sure you're using the right overlay for your environment.

## Getting Help

- [GitHub Issues](https://github.com/ugiordan/operator-chaos/issues) for bug reports
- [CLI Commands Reference](reference/cli-commands.md) for command syntax
- [Failure Modes Reference](failure-modes/index.md) for injection type parameters
