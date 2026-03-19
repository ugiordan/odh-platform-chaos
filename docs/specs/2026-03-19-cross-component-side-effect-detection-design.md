# Cross-Component Side-Effect Detection — Design Spec

**Jira:** RHOAIENG-52330 (8 SP)
**Date:** 2026-03-19
**Status:** Approved (post second architect review)

## Problem

When injecting chaos on a target component (e.g., killing kserve-controller-manager), dependent components (e.g., llmisvc-controller-manager, kserve-localmodel-controller-manager) may silently degrade. Today the orchestrator only observes the target component — collateral damage goes undetected.

The `Dependencies` field exists in `ComponentModel` and is populated in 6 of 7 knowledge files, but no Go code consumes it at runtime.

## Goals

- Detect unintended side effects on dependent components during chaos experiments
- Support both intra-operator (same knowledge file) and cross-operator (across files) dependencies
- Introduce the Blackboard observation pattern for extensible observation
- Add collateral findings to experiment reports and verdicts

## Non-Goals

- Transitive dependency traversal (direct dependents only)
- Load testing composition
- Cloud-provider-level fault injection

## Architecture

### Observation Pattern: Blackboard

The observation phase currently uses sequential, tightly-coupled calls in the orchestrator. This design replaces it with the Blackboard pattern:

```
                         ┌──────────────────────────────┐
                         │       ObservationBoard       │
                         │   (thread-safe findings       │
                         │    collector)                 │
                         └──────┬───────┬───────┬───────┘
                                │       │       │
                    ┌───────────┘       │       └───────────┐
                    ▼                   ▼                   ▼
          ReconciliationContributor  SteadyStateContributor  CollateralContributor
          (migrated from existing)   (migrated from existing)  (NEW)
```

Contributors implement `ObservationContributor` and write `Finding` entries to the shared `ObservationBoard`. The evaluator reads the board after all contributors complete.

**Two-phase execution:** Reconciliation determines whether the system recovered; steady-state checks verify the recovered state. These cannot run concurrently. The orchestrator runs contributors in two phases:

- **Phase 1 (blocking):** `ReconciliationContributor` runs alone. It polls managed resources until reconciled or timeout.
- **Phase 2 (concurrent):** `SteadyStateContributor` + `CollateralContributor` run in parallel via `RunContributors`. Both verify post-recovery state independently.

### Key Types

#### FindingSource (pkg/observer/board.go)

```go
type FindingSource string

const (
    SourceReconciliation FindingSource = "reconciliation"
    SourceSteadyState    FindingSource = "steady_state"
    SourceCollateral     FindingSource = "collateral"
)
```

#### ObservationBoard (pkg/observer/board.go)

```go
type ObservationBoard struct {
    mu       sync.Mutex
    findings []Finding
}

type Finding struct {
    Source    FindingSource          // typed constant
    Component string                // component name
    Operator  string                // operator name (for cross-operator findings)
    Passed   bool
    Details  string
    Checks   *v1alpha1.CheckResult  // for steady-state type findings
    ReconciliationResult *ReconciliationResult // for reconciliation findings
}

// AddFinding appends a finding under the mutex.
func (b *ObservationBoard) AddFinding(f Finding)

// Findings returns a COPY of the findings slice (safe to read after concurrent writes).
func (b *ObservationBoard) Findings() []Finding

// FindingsBySource returns a filtered copy, holding the mutex while iterating.
func (b *ObservationBoard) FindingsBySource(source FindingSource) []Finding
```

#### ObservationContributor (pkg/observer/contributor.go)

```go
type ObservationContributor interface {
    Observe(ctx context.Context, board *ObservationBoard) error
}

// RunContributors spawns goroutines for each contributor using sync.WaitGroup,
// waits for all to complete, and collects all errors in a mutex-guarded []error.
// One contributor's failure does NOT cancel siblings — all contributors run to
// completion. Do NOT use errgroup.WithContext (it cancels on first error).
func RunContributors(ctx context.Context, board *ObservationBoard, contributors []ObservationContributor) []error
```

#### Contributors

**ReconciliationContributor** — migrated from existing `ReconciliationChecker.CheckReconciliation()`. Polls managed resources until reconciled or timeout. Writes a single finding with `Source: SourceReconciliation` and embeds the `ReconciliationResult`. Runs in Phase 1 (blocking, alone).

**SteadyStateContributor** — migrated from existing `KubernetesObserver.CheckSteadyState()`. Runs the experiment's steady-state checks. Writes a finding with `Source: SourceSteadyState`. Runs in Phase 2.

**CollateralContributor** — NEW. Resolves dependent components from the dependency graph. Iterates dependents **sequentially** within its `Observe` call (simple, sufficient for V1). For each dependent, runs its `steadyState.checks` and writes findings with `Source: SourceCollateral`. Runs in Phase 2.

### Dependency Graph (pkg/model/graph.go)

```go
// ComponentRef is a struct key for type-safe component identification.
type ComponentRef struct {
    Operator  string
    Component string
}

type DependencyGraph struct {
    components map[ComponentRef]*ResolvedComponent
    // edges maps a dependency TARGET to its DEPENDENTS.
    // If component A depends on B, edges[B] contains A.
    edges      map[ComponentRef][]ComponentRef
}

type ResolvedComponent struct {
    Ref       ComponentRef
    Component *ComponentModel
    Namespace string
}

func BuildDependencyGraph(models []*OperatorKnowledge) (*DependencyGraph, error)
func (g *DependencyGraph) DirectDependents(ref ComponentRef) []*ResolvedComponent
```

**Edge direction:** If component A declares `dependencies: ["B"]`, then `BuildDependencyGraph` creates an edge FROM B TO A. Calling `DirectDependents(B)` returns `[A]` — components that depend on B, i.e., the ones that may degrade when B is faulted.

**Name resolution** (within `BuildDependencyGraph`):

1. Exact component name match within same operator → intra-operator dependency. Edge: `{op, depName} → {op, component}`.
2. Operator name match across loaded knowledge files → cross-operator dependency. Creates edges from each component in the matched operator (that has `steadyState.checks`) to the declaring component.
3. If a dependency string matches neither a component name in the same operator nor an operator name across loaded files, `BuildDependencyGraph` logs a warning and skips the edge (no error returned). This prevents typos from silently disabling collateral checking while keeping the graph build non-fatal.

**Deduplication:** `BuildDependencyGraph` deduplicates edges — if the same `{target → dependent}` edge would be created twice (e.g., from duplicate entries in `Dependencies`), only one edge is stored.

### Multi-File Knowledge Loading (pkg/model/loader.go)

```go
func LoadKnowledgeDir(dir string) ([]*OperatorKnowledge, error)
```

Reads all `*.yaml` files from a directory. Each file must parse as valid `OperatorKnowledge`. Returns all loaded models.

### Evaluator Changes (pkg/evaluator/engine.go)

Two independent methods sharing extracted helpers:

```go
// Existing method — preserved for backward compatibility.
// Uses computeVerdict() and collectDeviations() internally.
func (e *Evaluator) Evaluate(
    preCheck *v1alpha1.CheckResult,
    postCheck *v1alpha1.CheckResult,
    allReconciled bool,
    reconcileCycles int,
    recoveryTime time.Duration,
    hypothesis v1alpha1.HypothesisSpec,
) *EvaluationResult

// New method — accepts findings directly (not *ObservationBoard)
// to avoid import cycle evaluator → observer.
// Uses computeVerdict() and collectDeviations() internally,
// then applies collateral downgrade.
func (e *Evaluator) EvaluateFromFindings(
    findings []observer.Finding,
    hypothesis v1alpha1.HypothesisSpec,
) *EvaluationResult
```

`Evaluate()` and `EvaluateFromFindings()` are independent methods. They share extracted private helpers (`computeVerdict`, `collectDeviations`) but neither delegates to the other. This avoids lossy synthetic board construction.

**Import structure:** The `Finding` type is defined in `pkg/observer/board.go`. The evaluator imports `pkg/observer` for the `Finding` type only — no circular dependency since observer does not import evaluator.

**Collateral downgrade logic** (in `EvaluateFromFindings`):
1. Extract reconciliation and steady-state findings → pass to `computeVerdict` for existing verdict logic
2. Extract collateral findings → if any failed, downgrade `Resilient` to `Degraded`
3. Add `collateral_degradation` deviation for each failed collateral check

**Verdict impact:** collateral failures downgrade `Resilient` → `Degraded`, never to `Failed`. A collateral failure is a side effect, not the target's own failure.

### Report Changes (pkg/reporter/json.go)

```go
type ExperimentReport struct {
    // ... existing fields unchanged ...
    Collateral []CollateralFinding `json:"collateral,omitempty"`
}

type CollateralFinding struct {
    Operator  string                `json:"operator"`
    Component string                `json:"component"`
    Passed    bool                  `json:"passed"`
    Checks    *v1alpha1.CheckResult `json:"checks,omitempty"`
}
```

### Orchestrator Changes (pkg/orchestrator/lifecycle.go)

The observation phase (steps 4-5) is refactored into two phases:

```go
// Build observation board
board := observer.NewObservationBoard()

// === Phase 1: Reconciliation (blocking) ===
if component != nil && o.reconciler != nil {
    reconContributor := observer.NewReconciliationContributor(
        o.reconciler, component, namespace, recoveryTimeout)
    if err := reconContributor.Observe(ctx, board); err != nil {
        // log error, continue to phase 2
    }
}

// === Phase 2: Steady-state + Collateral (concurrent) ===
var phase2 []observer.ObservationContributor

// Steady-state post-check
if len(exp.Spec.SteadyState.Checks) > 0 {
    phase2 = append(phase2, observer.NewSteadyStateContributor(
        o.observer, exp.Spec.SteadyState.Checks, namespace))
}

// Collateral (if dependency graph available)
if o.depGraph != nil {
    ref := model.ComponentRef{
        Operator:  exp.Spec.Target.Operator,
        Component: exp.Spec.Target.Component,
    }
    dependents := o.depGraph.DirectDependents(ref)
    if len(dependents) > 0 {
        phase2 = append(phase2, observer.NewCollateralContributor(
            o.observer, dependents))
    }
}

// Run phase 2 contributors concurrently, log any errors
if len(phase2) > 0 {
    if errs := observer.RunContributors(ctx, board, phase2); len(errs) > 0 {
        for _, e := range errs {
            o.log("phase 2 contributor error: %v", e)
        }
    }
}

// Evaluate from findings
evalResult := o.evaluator.EvaluateFromFindings(board.Findings(), exp.Spec.Hypothesis)
```

### CLI Changes

Both `run` and `suite` commands gain:

```
--knowledge <file>       # now repeatable (StringArrayVar instead of StringVar)
--knowledge-dir <dir>    # loads all *.yaml from directory
```

When multiple knowledge files are provided (via either flag), the orchestrator builds a `DependencyGraph` and stores it in `OrchestratorConfig`.

### Orchestrator Config Changes

```go
type OrchestratorConfig struct {
    // ... existing fields ...
    DepGraph        *model.DependencyGraph      // NEW — nil when single knowledge file
    KnowledgeModels []*model.OperatorKnowledge   // NEW — all loaded models
}
```

## File Changes Summary

| File | Change |
|------|--------|
| `pkg/observer/board.go` | NEW — FindingSource, ObservationBoard, Finding |
| `pkg/observer/contributor.go` | NEW — ObservationContributor interface, RunContributors |
| `pkg/observer/reconciliation_contributor.go` | ADD ReconciliationContributor (existing checker preserved) |
| `pkg/observer/steadystate_contributor.go` | NEW — SteadyStateContributor wrapping KubernetesObserver |
| `pkg/observer/collateral_contributor.go` | NEW — CollateralContributor |
| `pkg/model/graph.go` | NEW — ComponentRef, DependencyGraph, BuildDependencyGraph |
| `pkg/model/loader.go` | ADD LoadKnowledgeDir |
| `pkg/evaluator/engine.go` | ADD EvaluateFromFindings (independent, shared helpers), extract computeVerdict/collectDeviations |
| `pkg/evaluator/types.go` | No change (Deviation type already flexible) |
| `pkg/reporter/json.go` | ADD CollateralFinding, extend ExperimentReport |
| `pkg/orchestrator/lifecycle.go` | REFACTOR observation phase to two-phase board + contributors |
| `internal/cli/orchestrator.go` | EXTEND for multi-knowledge, dep graph |
| `internal/cli/run.go` | ADD --knowledge-dir, make --knowledge repeatable |
| `internal/cli/suite.go` | ADD --knowledge-dir, make --knowledge repeatable |
| Tests for all new code | NEW |

## Testing Strategy

1. **Unit: DependencyGraph** — build from multiple knowledge models, verify intra/cross-operator resolution, verify direct-dependents-only traversal, verify edge direction (dependents of faulted component returned, not the other way around)
2. **Unit: ObservationBoard** — concurrent writes from multiple goroutines, FindingsBySource filtering with typed constants
3. **Unit: CollateralContributor** — mock observer, verify findings written to board with correct Source
4. **Unit: ReconciliationContributor** — verify existing behavior preserved via board
5. **Unit: SteadyStateContributor** — verify existing behavior preserved via board
6. **Unit: EvaluateFromFindings** — verdict × collateral downgrade matrix (8 combinations):
   | Base Verdict | Collateral | Result |
   |---|---|---|
   | Resilient | all pass | Resilient |
   | Resilient | any fail | Degraded |
   | Degraded | all pass | Degraded |
   | Degraded | any fail | Degraded |
   | Failed | all pass | Failed |
   | Failed | any fail | Failed |
   | Inconclusive | all pass | Inconclusive |
   | Inconclusive | any fail | Inconclusive |
7. **Unit: LoadKnowledgeDir** — loads all YAMLs from directory, rejects invalid files, skips non-YAML files
8. **Unit: Empty dependency graph** — no edges when no dependencies declared, DirectDependents returns nil
9. **Unit: Unresolvable dependency** — dependency string matches no component and no operator → warning logged, no edge created, no error returned
10. **Unit: Duplicate dependencies** — `dependencies: ["A", "A"]` produces only one edge, not duplicate collateral checks
11. **Unit: Single-file regression** — existing Evaluate() produces identical results as before refactoring
12. **Test fixtures** — 2-3 knowledge YAML files in testdata/ exercising:
    - Intra-operator deps (component A → component B, same file)
    - Cross-operator deps (component C depends on operator X)
    - Component with no deps (no edges created)
    - Component with deps but no steady-state checks (excluded from collateral)
13. **Integration: end-to-end** — load real knowledge files, build graph, run collateral contributor with fake client, verify report includes collateral findings

## Backward Compatibility

- Single `--knowledge` flag continues to work (no dependency graph, no collateral checks)
- `Evaluate()` method preserved as independent method (not delegating, shared helpers only)
- `ExperimentReport` JSON gains `collateral` field (omitempty — absent when no collateral checks run)
- Existing tests pass without modification
