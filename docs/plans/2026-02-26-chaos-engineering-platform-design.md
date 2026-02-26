# ODH Platform Chaos: Design Document

**Date**: 2026-02-26
**Status**: Approved
**Repository**: github.com/ugiordan/odh-platform-chaos

## Executive Summary

ODH Platform Chaos is a chaos engineering framework for the OpenDataHub ecosystem. It tests operator **reconciliation semantics** -- not just "does the pod restart?" but "did the operator recreate all managed resources with the correct configuration?"

No existing chaos tool does this. Litmus, Chaos Mesh, and Krkn operate at the infrastructure layer. This framework adds an **operator-semantic intelligence layer** that understands what operators manage and validates convergence behavior.

### Core Constraints

- The Go framework runs **deterministically** with no AI at runtime
- AI tools (Serena, council reasoning) operate **only during planning**
- The framework is **reproducible and auditable**
- Every experiment produces **structured, versioned artifacts**

---

## Architecture

### Three-Layer Design

```
+---------------------------------------------------------+
|               Plugin Layer (AI, optional)                |
|  Vendor-agnostic: Claude Code, Cursor, Windsurf, etc.   |
|  Uses current session model, no hardcoded model          |
|                                                         |
|  - AI council (challenger, security, coverage)           |
|  - Hypothesis generation from analysis results          |
|  - Automated fault point injection into source code     |
|  - Guardrails (schema, cross-validation, determinism)   |
|                                                         |
|  Calls odh-chaos CLI underneath                         |
+---------------------------------------------------------+
|              odh-chaos CLI (Go, deterministic)           |
|  Standalone, no AI dependency                           |
|                                                         |
|  - analyze   (static code analysis)                     |
|  - run       (execute experiment)                       |
|  - validate  (check experiment YAML)                    |
|  - discover  (find fault points on cluster)             |
|  - suite     (run multiple experiments)                 |
|  - report    (generate reports)                         |
|  - clean     (emergency stop)                           |
|  - init      (scaffold experiment YAML)                 |
|  - config    (user configuration)                       |
+---------------------------------------------------------+
|              Chaos SDK (Go library, optional)            |
|  Imported by component teams for deep instrumentation   |
|  Build-tag guarded, zero production overhead            |
|  Phase 2+ only -- not required for initial value        |
+---------------------------------------------------------+
```

### Phase-Based Rollout

Adoption is flipped: value first, SDK later.

**Phase 1 -- Zero-Code Chaos** (immediate value, no code changes):
- Static analysis produces resilience reports
- Infrastructure injection via client-go (PodKill, NetworkPartition, CRDMutation, ConfigDrift)
- ReconciliationChecker validates operator convergence
- Teams get actionable findings in 30 minutes

**Phase 2 -- Controller-Runtime Middleware** (one-line change):
- `chaos.WrapManager()` wraps the K8s client with fault injection
- Activated via ConfigMap, no fault points in business logic
- Covers: API errors, throttling, latency, watch disconnect, webhooks

**Phase 3 -- Full SDK + Plugin** (deep instrumentation):
- Fine-grained fault points in specific code paths
- AI plugin for council review and hypothesis generation
- Memory, CPU, I/O, concurrency faults for advanced teams

---

## Part 1: AI-Assisted Planning Phase

AI operates only during planning. It produces structured documentation artifacts
that guide the deterministic Go framework.

### 1.1 Codebase Analysis with Serena

During implementation, Serena analyzes each ODH component:

```
For each component in opendatahub-io:
  1. find_symbol("Reconcile") -> locate all reconcilers
  2. find_referencing_symbols(reconciler) -> map call graph
  3. For each reconciler:
     - Identify owned resources (Create/Update/Patch calls)
     - Identify error handling (error returns, retries)
     - Identify retry logic (ctrl.Result{RequeueAfter:})
     - Map dependencies (cross-namespace lookups)
     - Identify webhook handlers
     - Identify finalizer logic
  4. Output: knowledge YAML for that component
```

Serena provides token-efficient symbol-level analysis instead of reading
entire files, enabling rapid analysis across 30+ repositories.

### 1.2 Hypothesis Generation

Using knowledge model outputs, generate experiment hypotheses:

```markdown
## Hypothesis: Dashboard Pod Kill Recovery

**Source**: Analysis of dashboard_controller.go shows:
- Reconciler creates Deployment with 2 replicas
- RequeueAfter: 30s
- No explicit error handling for missing Deployment

**Steady state**: Deployment odh-dashboard has 2/2 ready replicas
**Injection**: Kill 1 of 2 pods
**Expected**: Pod recreated within 30s, all conditions restored
**Risk**: LOW
```

### 1.3 Council-Based Review

Multiple AI reasoning roles review each hypothesis before it becomes
an experiment. The council uses the current session's model by default.

**Council roles:**
- **Challenger**: Finds flaws in hypotheses, challenges assumptions
- **Security Reviewer**: Analyzes security implications and attack surfaces
- **Coverage Analyst**: Ensures experiments cover critical code paths

**Default behavior**: Uses whatever model the developer's coding agent runs.
No configuration needed.

**Custom configuration** (optional `council.yaml`):
```yaml
council:
  members:
    - role: challenger
      provider: anthropic
      model: claude-sonnet-4-20250514
    - role: security_reviewer
      provider: openai
      model: gpt-4o
    - role: coverage_analyst
      provider: ollama
      model: llama3.1:70b
  quorum: 3
  consensus: majority
```

### 1.4 Guardrails Framework

All AI outputs pass through guardrails before becoming artifacts:

**Schema enforcement**: Every output must conform to JSON Schema. Retry on failure.

**Cross-model consistency**: When multiple models review the same hypothesis,
compare outputs. Flag when verdicts disagree.

**Determinism enforcement (5-run check)**: Run the same prompt 5 times.
Classify consistency:
- 5/5 match (>= 0.95 similarity): highly deterministic
- 4/5 match (>= 0.80): acceptable
- 3/5 match (>= 0.60): borderline, requires human review
- 2/5 or worse: unreliable, rejected

**Evidence grounding**: Every claim must reference specific files or code.
Validator checks that referenced files exist.

**Human gate**: No AI-generated artifact enters the deterministic pipeline
without human approval. Full audit trail of every prompt and response.

### 1.5 CI Refresh Process

When operator code changes significantly:
1. Diff changed files
2. AI analyzes only changed reconcilers/controllers
3. Check if existing experiments still match the code
4. Propose new experiments for new code paths
5. Human reviews and merges updated experiments

---

## Part 2: Deterministic Go Framework

### 2.1 Operator Knowledge Model

Encodes operator semantics discovered during the AI planning phase.

```go
// pkg/model/knowledge.go

type OperatorKnowledge struct {
    Operator   OperatorMeta         `yaml:"operator"`
    Components []ComponentModel     `yaml:"components"`
    Recovery   RecoveryExpectations `yaml:"recovery"`
}

type ComponentModel struct {
    Name             string            `yaml:"name"`
    Controller       string            `yaml:"controller"`
    ManagedResources []ManagedResource `yaml:"managedResources"`
    Dependencies     []string          `yaml:"dependencies"`
    SteadyState      SteadyStateSpec   `yaml:"steadyState"`
    Webhooks         []WebhookSpec     `yaml:"webhooks"`
    Finalizers       []string          `yaml:"finalizers"`
    ConditionalResources []ConditionalResource `yaml:"conditionalResources"`
}

type ManagedResource struct {
    APIVersion string            `yaml:"apiVersion"`
    Kind       string            `yaml:"kind"`
    Name       string            `yaml:"name"`
    Namespace  string            `yaml:"namespace"`
    Labels     map[string]string `yaml:"labels"`
    OwnerRef   string            `yaml:"ownerRef"`
    ExpectedSpec map[string]interface{} `yaml:"expectedSpec"`
}

type ConditionalResource struct {
    Resource  ManagedResource `yaml:"resource"`
    Condition string          `yaml:"condition"`
}

type SteadyStateSpec struct {
    Checks  []SteadyStateCheck `yaml:"checks"`
    Timeout metav1.Duration    `yaml:"timeout"`
}

type SteadyStateCheck struct {
    Type          string `yaml:"type"` // resourceExists, conditionTrue, prometheusQuery
    APIVersion    string `yaml:"apiVersion,omitempty"`
    Kind          string `yaml:"kind,omitempty"`
    Name          string `yaml:"name,omitempty"`
    Namespace     string `yaml:"namespace,omitempty"`
    ConditionType string `yaml:"conditionType,omitempty"`
    Query         string `yaml:"query,omitempty"`
    Operator      string `yaml:"operator,omitempty"`
    Value         string `yaml:"value,omitempty"`
    For           string `yaml:"for,omitempty"`
}

type RecoveryExpectations struct {
    ReconcileTimeout metav1.Duration `yaml:"reconcileTimeout"`
    MaxReconcileCycles int           `yaml:"maxReconcileCycles"`
}
```

Example:
```yaml
operator:
  name: opendatahub-operator
  namespace: opendatahub
  repository: https://github.com/opendatahub-io/opendatahub-operator

components:
  - name: dashboard
    controller: DataScienceCluster
    managedResources:
      - apiVersion: apps/v1
        kind: Deployment
        name: odh-dashboard
        namespace: opendatahub
        labels:
          app.kubernetes.io/part-of: dashboard
        ownerRef: Dashboard
        expectedSpec:
          replicas: 2
      - apiVersion: v1
        kind: Service
        name: odh-dashboard
        namespace: opendatahub
        ownerRef: Dashboard
    dependencies: []
    finalizers:
      - dashboard.opendatahub.io/finalizer
    steadyState:
      checks:
        - type: resourceExists
          apiVersion: apps/v1
          kind: Deployment
          name: odh-dashboard
          namespace: opendatahub
        - type: conditionTrue
          apiVersion: apps/v1
          kind: Deployment
          name: odh-dashboard
          conditionType: Available
        - type: prometheusQuery
          query: "up{job='odh-dashboard'} == 1"
          for: 30s
      timeout: 120s

recovery:
  reconcileTimeout: 300s
  maxReconcileCycles: 10
```

### 2.2 Experiment Specification (CRD-Ready)

```go
// api/v1alpha1/chaosexperiment_types.go

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Verdict",type=string,JSONPath=`.status.verdict`
type ChaosExperiment struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              ChaosExperimentSpec   `json:"spec"`
    Status            ChaosExperimentStatus `json:"status,omitempty"`
}

type ChaosExperimentSpec struct {
    Target      TargetSpec      `json:"target" yaml:"target"`
    SteadyState SteadyStateDef  `json:"steadyState" yaml:"steadyState"`
    Injection   InjectionSpec   `json:"injection" yaml:"injection"`
    Observation ObservationSpec `json:"observation" yaml:"observation"`
    BlastRadius BlastRadiusSpec `json:"blastRadius" yaml:"blastRadius"`
    Hypothesis  HypothesisSpec  `json:"hypothesis" yaml:"hypothesis"`
}

type InjectionSpec struct {
    Type       InjectionType     `json:"type" yaml:"type"`
    Parameters map[string]string `json:"parameters" yaml:"parameters"`
    Duration   metav1.Duration   `json:"duration" yaml:"duration"`
    Count      int               `json:"count" yaml:"count"`
    TTL        metav1.Duration   `json:"ttl" yaml:"ttl"`
    DangerLevel string           `json:"dangerLevel,omitempty" yaml:"dangerLevel"`
}

type BlastRadiusSpec struct {
    MaxPodsAffected    int      `json:"maxPodsAffected" yaml:"maxPodsAffected"`
    MaxConcurrentFaults int     `json:"maxConcurrentFaults" yaml:"maxConcurrentFaults"`
    AllowedNamespaces  []string `json:"allowedNamespaces" yaml:"allowedNamespaces"`
    ForbiddenResources []string `json:"forbiddenResources" yaml:"forbiddenResources"`
    RequireLabel       string   `json:"requireLabel,omitempty" yaml:"requireLabel"`
    DryRun             bool     `json:"dryRun" yaml:"dryRun"`
}
```

**Phase 1 Injection Types** (infrastructure, no SDK needed):
```go
const (
    PodKill          InjectionType = "PodKill"
    PodFailure       InjectionType = "PodFailure"
    NetworkPartition InjectionType = "NetworkPartition"
    NetworkLatency   InjectionType = "NetworkLatency"
    ResourceExhaustion InjectionType = "ResourceExhaustion"
    CRDMutation      InjectionType = "CRDMutation"
    ConfigDrift      InjectionType = "ConfigDrift"
    WebhookDisrupt   InjectionType = "WebhookDisrupt"
    RBACRevoke       InjectionType = "RBACRevoke"
    FinalizerBlock   InjectionType = "FinalizerBlock"
    OwnerRefOrphan   InjectionType = "OwnerRefOrphan"
)
```

**Phase 2 Injection Types** (middleware, one-line change):
```go
const (
    SourceHook       InjectionType = "SourceHook"
    ClientThrottle   InjectionType = "ClientThrottle"
    APIServerError   InjectionType = "APIServerError"
    WatchDisconnect  InjectionType = "WatchDisconnect"
    LeaderElectionLoss InjectionType = "LeaderElectionLoss"
    WebhookTimeout   InjectionType = "WebhookTimeout"
    WebhookReject    InjectionType = "WebhookReject"
)
```

**Phase 3 Injection Types** (full SDK):
```go
const (
    MemoryLeak       InjectionType = "MemoryLeak"
    MemoryPressure   InjectionType = "MemoryPressure"
    GoroutineBomb    InjectionType = "GoroutineBomb"
    CPUSpin          InjectionType = "CPUSpin"
    FDExhaustion     InjectionType = "FDExhaustion"
    DiskWriteFailure InjectionType = "DiskWriteFailure"
    DNSFailure       InjectionType = "DNSFailure"
    DeadlockInject   InjectionType = "DeadlockInject"
)
```

Example experiment:
```yaml
apiVersion: chaos.opendatahub.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: dashboard-pod-kill-recovery
  labels:
    component: dashboard
    severity: standard
spec:
  target:
    operator: opendatahub-operator
    component: dashboard
    resource: Deployment/odh-dashboard

  hypothesis:
    description: "Dashboard recovers from pod termination within 60s"
    expectedBehavior: >
      Operator detects missing pod, Deployment controller creates
      replacement, all replicas reach Ready state
    recoveryTimeout: 60s

  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: odh-dashboard
        conditionType: Available
      - type: prometheusQuery
        query: "kube_deployment_status_replicas_ready{deployment='odh-dashboard'}"
        operator: ">="
        value: "2"
    timeout: 30s

  injection:
    type: PodKill
    parameters:
      signal: "SIGKILL"
      labelSelector: "app.kubernetes.io/part-of=dashboard"
    count: 1
    duration: 0s
    ttl: 300s

  observation:
    interval: 5s
    duration: 120s
    trackReconcileCycles: true

  blastRadius:
    maxPodsAffected: 1
    maxConcurrentFaults: 1
    allowedNamespaces: ["opendatahub"]
    dryRun: false
```

### 2.3 Injection Engine

```go
// pkg/injection/engine.go

type Injector interface {
    Validate(ctx context.Context, spec InjectionSpec, blast BlastRadiusSpec) error
    Inject(ctx context.Context, spec InjectionSpec) (CleanupFunc, []InjectionEvent, error)
}

type CleanupFunc func(ctx context.Context) error
```

Built-in injectors (Phase 1, no SDK):

| Injector | Mechanism | Cleanup |
|---|---|---|
| PodKillInjector | `client.Delete(pod)` with GracePeriodSeconds: 0 | None (controller recreates) |
| NetworkPartitionInjector | Create NetworkPolicy blocking traffic | Delete NetworkPolicy |
| CRDMutationInjector | Patch managed CR spec field | Restore original or let operator reconcile |
| ConfigDriftInjector | Patch managed ConfigMap/Secret data | Restore original or let operator reconcile |
| WebhookDisruptInjector | Modify webhook config (failurePolicy, timeout) | Restore original |
| RBACRevokeInjector | Remove permissions from ClusterRoleBinding | Restore original |
| FinalizerBlockInjector | Add a stuck finalizer to managed resource | Remove finalizer |
| OwnerRefOrphanInjector | Remove ownerReferences from child resources | Operator should re-establish |

Safety enforcement:
```go
func (e *Engine) Run(ctx context.Context, spec InjectionSpec, blast BlastRadiusSpec) error {
    // 1. Mutual exclusion: check for other active experiments
    if err := e.acquireLock(ctx, spec.Target); err != nil {
        return fmt.Errorf("another experiment is active: %w", err)
    }
    defer e.releaseLock(ctx, spec.Target)

    // 2. Blast radius validation
    affected, _ := e.countAffected(ctx, spec)
    if affected > blast.MaxPodsAffected {
        return fmt.Errorf("blast radius exceeded: %d > %d", affected, blast.MaxPodsAffected)
    }

    // 3. Namespace check
    if !contains(blast.AllowedNamespaces, spec.Namespace) {
        return fmt.Errorf("namespace %s not allowed", spec.Namespace)
    }

    // 4. Danger level check for resource-consuming faults
    if spec.DangerLevel == "high" && !blast.AllowDangerous {
        return fmt.Errorf("dangerous fault requires explicit opt-in")
    }

    // 5. Dry run check
    if blast.DryRun {
        log.Info("DRY RUN", "type", spec.Type, "target", spec.Target)
        return nil
    }

    // 6. Execute with TTL-guarded cleanup
    cleanup, events, err := e.injector.Inject(ctx, spec)
    e.registerCleanupWithTTL(cleanup, spec.TTL)
    return err
}
```

### 2.4 Observer Engine

```go
// pkg/observer/engine.go

type Observer interface {
    CheckSteadyState(ctx context.Context, spec SteadyStateDef) (*CheckResult, error)
    Watch(ctx context.Context, spec ObservationSpec) (<-chan Observation, error)
}

type CompositeObserver struct {
    prom *PrometheusObserver
    k8s  *KubernetesObserver
    recon *ReconciliationChecker
}
```

**ReconciliationChecker** -- the key innovation:
```go
// pkg/observer/reconciliation.go

type ReconciliationChecker struct {
    client    client.Client
    knowledge *OperatorKnowledge
}

func (r *ReconciliationChecker) CheckReconciliation(
    ctx context.Context,
    component string,
    timeout time.Duration,
) (*ReconciliationResult, error) {
    model := r.knowledge.GetComponent(component)
    startTime := time.Now()
    var cycles int

    return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true,
        func(ctx context.Context) (bool, error) {
            cycles++
            for _, res := range model.ManagedResources {
                obj := &unstructured.Unstructured{}
                obj.SetAPIVersion(res.APIVersion)
                obj.SetGroupVersionKind(/* from res */)

                err := r.client.Get(ctx, types.NamespacedName{
                    Name: res.Name, Namespace: res.Namespace,
                }, obj)
                if err != nil {
                    return false, nil // not yet reconciled
                }

                // Verify metadata (labels, ownerRefs)
                if !r.matchesMetadata(obj, res) {
                    return false, nil
                }

                // Verify spec (deep comparison against expected)
                if !r.matchesSpec(obj, res.ExpectedSpec) {
                    return false, nil
                }

                // Verify conditions
                if !r.matchesConditions(obj, model.SteadyState) {
                    return false, nil
                }
            }
            return true, nil
        })

    return &ReconciliationResult{
        AllReconciled:   allPassed,
        RecoverTime:     time.Since(startTime),
        ReconcileCycles: cycles,
    }
}
```

This checks resource existence, metadata, spec correctness, AND conditions --
not just "is the pod running."

### 2.5 Evaluator Engine

```go
// pkg/evaluator/engine.go

type EvaluationResult struct {
    Verdict          Verdict         `json:"verdict"`
    Confidence       string          `json:"confidence"`
    RecoveryTime     time.Duration   `json:"recoveryTime"`
    ReconcileCycles  int             `json:"reconcileCycles"`
    SteadyStatePre   CheckResult     `json:"steadyStatePre"`
    SteadyStatePost  CheckResult     `json:"steadyStatePost"`
    Reconciliation   ReconcileResult `json:"reconciliation"`
    Deviations       []Deviation     `json:"deviations"`
}

func (e *Evaluator) Evaluate(...) *EvaluationResult {
    result := &EvaluationResult{}

    // 1. Baseline established?
    if !preCheck.Passed {
        result.Verdict = Inconclusive
        result.Confidence = "0 steady-state checks failed before injection"
        return result
    }

    // 2. Recovered?
    if postCheck.Passed && reconciliation.AllReconciled {
        result.Verdict = Resilient
    } else if postCheck.Passed && !reconciliation.AllReconciled {
        result.Verdict = Degraded
    } else {
        result.Verdict = Failed
    }

    // 3. Recovery time within hypothesis?
    if result.RecoveryTime > hypothesis.RecoveryTimeout.Duration {
        if result.Verdict == Resilient {
            result.Verdict = Degraded
        }
    }

    // 4. Excessive reconcile cycles?
    if result.ReconcileCycles > knowledge.Recovery.MaxReconcileCycles {
        result.Deviations = append(result.Deviations, Deviation{
            Type: "excessive_reconciliation",
            Detail: fmt.Sprintf("%d cycles (max %d)",
                result.ReconcileCycles,
                knowledge.Recovery.MaxReconcileCycles),
        })
    }

    // 5. Confidence qualifier
    result.Confidence = fmt.Sprintf(
        "%d steady-state checks passed, %s recovery, %d reconcile cycles",
        postCheck.ChecksPassed, result.RecoveryTime, result.ReconcileCycles)

    return result
}
```

Verdicts:
- **Resilient**: Recovered within timeout, all resources reconciled correctly
- **Degraded**: Recovered but too slowly, or excessive reconcile cycles
- **Failed**: Not recovered, resources not reconciled
- **Inconclusive**: Couldn't establish baseline steady state

### 2.6 Reporter Engine

```go
type Reporter interface {
    Report(ctx context.Context, result ExperimentResult) error
}

// JSONReporter: structured JSON to file or stdout
// JUnitReporter: JUnit XML for CI integration
// K8sEventReporter: Kubernetes events on the experiment target
```

Example output:
```json
{
  "experiment": "dashboard-pod-kill-recovery",
  "timestamp": "2026-02-26T10:15:00Z",
  "verdict": "Resilient",
  "confidence": "3 steady-state checks passed, 12.3s recovery, 2 reconcile cycles",
  "hypothesis": "Dashboard recovers from pod termination within 60s",
  "target": {
    "operator": "opendatahub-operator",
    "component": "dashboard",
    "resource": "Deployment/odh-dashboard"
  },
  "injection": {
    "type": "PodKill",
    "podsKilled": ["odh-dashboard-7f8b9c-x4k2n"],
    "timestamp": "2026-02-26T10:15:05Z"
  },
  "recovery": {
    "timeToRecover": "12.3s",
    "reconcileCycles": 2,
    "reconciled": true,
    "resourcesVerified": [
      {"kind": "Deployment", "name": "odh-dashboard", "status": "Available", "specMatch": true},
      {"kind": "Service", "name": "odh-dashboard", "status": "Active", "specMatch": true}
    ]
  },
  "steadyState": {
    "pre": {"passed": true, "checksRun": 3, "checksPassed": 3},
    "post": {"passed": true, "checksRun": 3, "checksPassed": 3}
  },
  "observations": [
    {"time": "T+0s", "replicas_ready": 2},
    {"time": "T+5s", "replicas_ready": 1},
    {"time": "T+10s", "replicas_ready": 1},
    {"time": "T+15s", "replicas_ready": 2}
  ]
}
```

### 2.7 Experiment Lifecycle

```
    +----------+
    | PENDING  |  Experiment loaded, validated
    +----+-----+
         | validate spec + blast radius + mutual exclusion
    +----v---------------+
    | STEADY_STATE_PRE   |  Verify system healthy before injection
    +----+---------------+
         | all checks pass (or ABORT if unhealthy)
    +----v------+
    | INJECTING |  Execute fault injection
    +----+------+
         | injection confirmed (acknowledgment received)
    +----v-------+
    | OBSERVING  |  Collect metrics, watch K8s state, record timeline
    +----+-------+
         | observation window complete
    +----v----------------+
    | STEADY_STATE_POST   |  Verify system recovered
    +----+----------------+
         |
    +----v--------+
    | EVALUATING  |  Compare pre/post, count reconcile cycles, verdict
    +----+--------+
         |
    +----v------+
    | CLEANUP   |  Deactivate faults, restore state, release lock
    +----+------+
         |
    +----v------+
    | COMPLETE  |  Verdict: RESILIENT / DEGRADED / FAILED / INCONCLUSIVE
    +-----------+

Any phase can transition to ABORTED if safety constraints are violated.
TTL watchdog auto-cleans if CLI crashes.
```

### 2.8 Static Analyzer

Uses `go/ast` for reliable syntactic analysis. Detects:

1. **K8s API calls**: `client.Get`, `client.Create`, `client.Update`, `client.Delete`, `client.Patch`
2. **Ignored errors**: Error returns assigned to `_`
3. **Goroutine creation**: `go func()` launch points
4. **Context usage**: Functions accepting `context.Context` (deadline injection candidates)
5. **Network calls**: HTTP client patterns, gRPC dial patterns, database connections

Output: fault point candidates ranked by severity, with file/line/function location
and suggested fault type. Can generate ready-to-apply patches.

```bash
odh-chaos analyze /path/to/model-registry
# +------+------------------------+----------+---------------------------+
# | Sev  | Location               | Category | Reason                    |
# +------+------------------------+----------+---------------------------+
# | CRIT | pkg/db/connect.go:45   | network  | sql.Open error ignored    |
# | HIGH | pkg/api/handler.go:120 | security | JSON unmarshal, no limit  |
# | HIGH | pkg/cache/store.go:78  | memory   | Unbounded append in loop  |
# | MED  | pkg/sync/worker.go:55  | concurr  | Mutex held across HTTP    |
# +------+------------------------+----------+---------------------------+

odh-chaos analyze /path/to/model-registry --generate-experiments
# Produces: experiments/model-registry-generated.yaml
```

### 2.9 CLI Design

```
odh-chaos
+-- run          Run a single experiment from YAML
+-- suite        Run a suite of experiments
+-- validate     Validate experiment YAML without running
+-- analyze      Static analysis of Go source code
+-- discover     Find fault points from running components (via HTTP)
+-- status       Check active faults and experiments
+-- clean        Emergency stop: remove all active faults
+-- report       Generate reports from stored results
+-- init         Scaffold a new experiment YAML
+-- config       User configuration (set model, set defaults)
```

Usage:
```bash
# Phase 1: zero-code experiments
odh-chaos run experiments/dashboard-pod-kill.yaml --kubeconfig ~/.kube/config
odh-chaos run experiments/dashboard-pod-kill.yaml --dry-run
odh-chaos suite experiments/ --parallel=3 --report-dir=./results
odh-chaos clean --namespace opendatahub

# Analysis
odh-chaos analyze /path/to/operator
odh-chaos analyze /path/to/operator --generate-experiments

# Discovery (Phase 2+, requires SDK)
odh-chaos discover --namespace opendatahub

# Configuration
odh-chaos config set model claude-sonnet-4-20250514
```

### 2.10 Safety Mechanisms

**Fault TTL**: Every fault activation includes a TTL. The SDK independently
enforces this regardless of CLI state. If the CLI crashes, faults auto-disable.

**Experiment mutual exclusion**: Lease-based locking ensures only one experiment
runs per operator at a time. Prevents unpredictable fault composition.

**Activation acknowledgment**: After the CLI writes a ConfigMap, the target
component must acknowledge receipt. The CLI waits with timeout before
transitioning to OBSERVING.

**Magnitude bounds**: Resource-consuming faults (memory, CPU) require explicit
magnitude parameters with hard upper bounds. The SDK refuses unbounded
consumption.

**Startup warning**: When chaos hooks are active, the SDK logs:
`CHAOS ENGINEERING HOOKS ACTIVE -- THIS BUILD IS NOT FOR PRODUCTION`

**HTTP endpoint security**: Admin endpoint requires auth token, binds to
localhost by default. Only accessible via port-forward.

**Emergency stop**: `odh-chaos clean` scans all ConfigMaps with chaos labels
and disables all faults. Works even without knowledge of running experiments.

---

## Part 2.5: Chaos SDK (Phase 2+)

### Controller-Runtime Middleware (Phase 2)

One-line integration, no fault points in business logic:

```go
// In SetupWithManager -- the only change needed
func (r *DashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {
    reconciler := &DashboardReconciler{Client: mgr.GetClient()}

    // One-line chaos wrapping
    wrapped := chaos.WrapReconciler(reconciler, chaos.WithClientFaults())

    return ctrl.NewControllerManagedBy(mgr).
        For(&v1.Dashboard{}).
        Owns(&appsv1.Deployment{}).
        Complete(wrapped)
}
```

The wrapped client intercepts K8s API calls and can inject errors, throttling,
latency, and watch disconnects based on ConfigMap configuration.

### Hybrid Client Wrapper

For K8s-specific faults, use a client wrapper instead of build tags. The overhead
of a nil-check passthrough is negligible (nanoseconds):

```go
type ChaosClient struct {
    inner  client.Client
    faults *FaultConfig  // nil by default = pure passthrough
}

func (c *ChaosClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
    if c.faults != nil {
        if err := c.faults.MaybeInject("get"); err != nil {
            return err
        }
    }
    return c.inner.Get(ctx, key, obj, opts...)
}
```

Reserve build tags only for heavyweight faults (memory, CPU, goroutines)
that have real overhead even when inactive.

### Full SDK Fault Points (Phase 3)

For teams that need deep instrumentation beyond the client wrapper:

```go
ch := chaos.New(chaos.Config{
    Component: "model-registry",
    Namespace: "opendatahub",
    Client:    mgr.GetClient(),
    AdminPort: 9090,
})

fpDB := ch.Register("db.connect", faults.Network)
fpAuth := ch.Register("api.auth", faults.Application)

// In code
if ferr := fpDB.ForceError(); ferr != nil {
    return nil, ferr
}

// Standalone testing (no cluster needed)
func TestDBFailure(t *testing.T) {
    ch := chaos.NewForTest(t, "model-registry")
    ch.Activate("model-registry.db.connect", faults.ForceErrorConfig{
        Error: "connection refused",
    })
    err := registry.Connect(ctx)
    assert.Error(t, err)
}
```

Fault point discovery via HTTP:
```bash
GET /chaos/faultpoints
[
  {"name": "model-registry.db.connect", "category": "network"},
  {"name": "model-registry.api.auth", "category": "application"}
]
```

### Fault Categories by Phase

| Phase | Category | Faults |
|---|---|---|
| 1 | Infrastructure | PodKill, NetworkPartition, CRDMutation, ConfigDrift, WebhookDisrupt, RBACRevoke, FinalizerBlock, OwnerRefOrphan |
| 2 | Application | ForceError, Skip, Panic |
| 2 | Timing | Delay, Jitter, DeadlineExceed |
| 2 | Kubernetes | ClientThrottle, APIServerError, WatchDisconnect, LeaderElectionLoss |
| 2 | Webhook | WebhookTimeout, WebhookReject, CertExpiry |
| 3 | Network | ConnectionPoolExhaust, DNSFailure, SocketTimeout |
| 3 | Memory | LeakMemory, MemoryPressure, AllocSpike |
| 3 | CPU | GoroutineBomb, BusySpin, GCPressure |
| 3 | I/O | ExhaustFileDescriptors, DiskWriteFailure, SlowReader |
| 3 | Concurrency | DeadlockInject, ChannelBlock, MutexStarvation |

---

## Part 3: Validation Strategy

### 3.1 AI Hypotheses to Go Experiments

```
AI hypothesis (markdown)
    | human reviews
Approved hypothesis
    | AI generates or human writes
Experiment YAML (committed to repo)
    | odh-chaos validate
Validated experiment
    | odh-chaos run
Deterministic execution (no AI)
    |
Structured JSON result
```

Key guarantee: once an experiment YAML is committed, its execution is
100% deterministic. Same YAML + same cluster state = same result.

### 3.2 Human Validation Checkpoints

1. **Knowledge model review**: Human verifies generated YAML matches
   their understanding of the operator
2. **Experiment review**: Human verifies correct steady-state definitions,
   safe blast radius, reasonable timeouts
3. **Result review**: Human reviews results, especially INCONCLUSIVE
   verdicts that may indicate bad experiment design
4. **Regression tracking**: Experiments versioned in git alongside operator code

### 3.3 Versioning and Regression

```
odh-platform-chaos/
+-- knowledge/
|   +-- opendatahub-operator.yaml
|   +-- model-registry.yaml
|   +-- odh-model-controller.yaml
+-- experiments/
|   +-- operator/
|   |   +-- dashboard-pod-kill.yaml
|   |   +-- kserve-reconcile-drift.yaml
|   +-- model-registry/
|   |   +-- db-connection-failure.yaml
|   +-- model-controller/
|       +-- inference-route-failure.yaml
+-- results/
|   +-- 2026-02-26/
|   |   +-- dashboard-pod-kill.json
|   |   +-- summary.json
|   +-- latest -> 2026-02-26/
+-- docs/
    +-- plans/
    +-- reviews/
```

Experiments pinned to operator versions. CI refresh flags experiments
that need updating when operator code changes.

### 3.4 Runtime AI Independence

The system guarantees no AI at runtime through architectural separation:
- Experiment YAML is the only interface between AI (planning) and Go (runtime)
- The CLI has no LLM client, no API keys, no model configuration
- All AI capabilities live in the plugin layer, which is optional
- The plugin produces artifacts (YAML, markdown); the CLI consumes them

---

## Risk Analysis

| Risk | Severity | Mitigation |
|---|---|---|
| Adoption resistance from teams | High | Phase 1 requires zero code changes; demonstrate value first |
| Fault escapes blast radius | High | Pre-flight validation, mutual exclusion, TTL watchdog |
| HTTP endpoint exploited in staging | High | Auth token required, localhost binding, build-tag guard |
| Knowledge model becomes stale | Medium | CI refresh process, version pinning |
| Memory/CPU faults crash node | Medium | Magnitude bounds, resource limit verification, danger level flag |
| Cleanup fails after CLI crash | Medium | TTL auto-disable in SDK, emergency `clean` command |
| Static analyzer noise kills trust | Medium | Scope to reliable AST patterns, severity ranking |
| False confidence from "Resilient" verdict | Medium | Confidence qualifier on every verdict |
| ConfigMap race between experiments | Medium | Lease-based mutual exclusion |
| SDK dependency blocks operator upgrades | Low | Minimal API surface, semantic versioning |

---

## Implementation Milestones

| Milestone | Deliverable | Phase |
|---|---|---|
| M1 | CLI scaffold + experiment types + lifecycle state machine | 1 |
| M2 | Knowledge model loader + YAML schema | 1 |
| M3 | Infrastructure injectors (PodKill, NetworkPartition, CRDMutation, ConfigDrift) | 1 |
| M4 | Observer engine (Prometheus + K8s watch + ReconciliationChecker) | 1 |
| M5 | Evaluator + Reporter (JSON + JUnit) + dry-run mode | 1 |
| M6 | Static analyzer (go/ast, candidate detection, patch generation) | 1 |
| M7 | AI-assisted knowledge model generation for ODH operator | 1 |
| M8 | First 10 experiments for ODH operator + results | 1 |
| M9 | Chaos SDK core + controller-runtime middleware + hybrid client | 2 |
| M10 | ConfigMap activation + HTTP endpoint + Go test helper | 2 |
| M11 | Application + Timing + K8s + Webhook fault categories | 2 |
| M12 | Safety mechanisms (TTL, mutual exclusion, acknowledgment) | 2 |
| M13 | Advanced injectors (WebhookDisrupt, RBACRevoke, FinalizerBlock) | 2 |
| M14 | AI plugin: council framework, guardrails, hypothesis generation | 3 |
| M15 | Memory + CPU + I/O + Concurrency fault categories | 3 |
| M16 | Suite runner + CI integration + GitHub Actions templates | 3 |
| M17 | Controller mode (ChaosExperiment CRD) | 3 |

---

## Differentiation from Existing Tools

| Capability | Krkn | Litmus | Chaos Mesh | ODH Platform Chaos |
|---|---|---|---|---|
| Pod kill | Yes | Yes | Yes | Yes |
| Network chaos | Yes | Yes | Yes | Yes |
| Node disruption | Yes | Yes | No | No (not needed) |
| CRD mutation | No | No | No | **Yes** |
| Config drift detection | No | No | No | **Yes** |
| Reconciliation verification | No | No | No | **Yes** |
| Operator knowledge model | No | No | No | **Yes** |
| In-process fault injection | No | No | No | **Yes** (Phase 2) |
| Static code analysis | No | No | No | **Yes** |
| Webhook chaos | No | Partial | No | **Yes** |
| AI-assisted planning | Krkn-AI (metrics) | No | No | **Yes** (code-aware) |
| Source code changes needed | No | No | No | No (Phase 1) / Optional (Phase 2+) |
