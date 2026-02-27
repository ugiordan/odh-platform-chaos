# ODH Platform Chaos

Chaos engineering framework for OpenDataHub operators. Tests operator reconciliation semantics --- not just that pods restart, but that operators correctly restore all managed resources.

## Why ODH Platform Chaos?

Existing chaos tools (Krkn, Litmus, Chaos Mesh) test infrastructure resilience: kill a pod, verify it comes back. But Kubernetes operators manage complex resource graphs --- Deployments, Services, ConfigMaps, CRDs --- where the real question is:

**"When something breaks, does the operator put everything back the way it should be?"**

ODH Platform Chaos answers this by:
- **Testing reconciliation**: Verifying operators restore resources to their intended state
- **Operator-semantic faults**: CRD mutation and config drift --- faults specific to operators
- **Knowledge-driven**: Understanding what each operator manages via knowledge models
- **Structured verdicts**: Resilient, Degraded, Failed, or Inconclusive

## Quick Start

### Install

```bash
go install github.com/opendatahub-io/odh-platform-chaos/cmd/odh-chaos@latest
```

### Run Your First Experiment

1. Create an experiment:
```bash
odh-chaos init --component dashboard --type PodKill > experiment.yaml
```

2. Validate:
```bash
odh-chaos validate experiment.yaml
```

3. Dry run:
```bash
odh-chaos run experiment.yaml --dry-run
```

4. Execute (requires cluster access):
```bash
odh-chaos run experiment.yaml --knowledge knowledge/odh-operator.yaml
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `run` | Execute a chaos experiment |
| `validate` | Validate experiment YAML without running |
| `init` | Generate skeleton experiment YAML |
| `clean` | Emergency removal of chaos artifacts |
| `analyze` | Scan Go source for fault injection candidates |
| `suite` | Run all experiments in a directory |
| `report` | Generate summary reports from results |

### Run

```bash
odh-chaos run experiment.yaml [flags]
```

Flags:
- `--knowledge` --- Path to operator knowledge YAML
- `--report-dir` --- Directory for JSON report output
- `--dry-run` --- Validate without injecting faults

### Analyze

```bash
odh-chaos analyze /path/to/operator [flags]
```

Scans Go source code for fault injection candidates:
- Ignored errors
- Goroutine launches
- Network calls
- Database calls
- K8s API calls

## Experiment Format

```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: dashboard-pod-kill
spec:
  target:
    operator: opendatahub-operator
    component: dashboard
  hypothesis:
    description: "Dashboard recovers within 60s"
    recoveryTimeout: "60s"
  injection:
    type: PodKill
    count: 1
    ttl: "300s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces: [opendatahub]
```

### Injection Types

| Type | Description |
|------|-------------|
| PodKill | Delete pods matching selector |
| NetworkPartition | Block traffic via NetworkPolicy |
| CRDMutation | Mutate managed CR spec fields |
| ConfigDrift | Modify managed ConfigMap/Secret data |

### Verdicts

| Verdict | Meaning |
|---------|---------|
| Resilient | Recovered within timeout, all resources reconciled |
| Degraded | Recovered but slow, partial reconciliation, or excessive cycles |
| Failed | Did not recover or steady-state checks failed |
| Inconclusive | Could not establish baseline |

## Architecture

```
+--------------+
|  CLI Layer   |  odh-chaos run/validate/init/clean/analyze/suite/report
+--------------+
| Orchestrator |  Lifecycle: Validate -> PreCheck -> Inject -> Observe -> PostCheck -> Evaluate
+--------------+
|   Engines    |  Injection | Observer | Evaluator | Reporter | Safety
+--------------+
|  Knowledge   |  Operator models (YAML) + Experiment specs (YAML)
+--------------+
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests first (TDD)
4. Submit a pull request

## License

Apache License 2.0
