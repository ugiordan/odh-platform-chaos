package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/evaluator"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/injection"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/observer"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/reporter"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/safety"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultCleanupTimeout  = 30 * time.Second
	defaultRecoveryTimeout = 60 * time.Second
)

// Orchestrator wires together all engines and manages the experiment
// lifecycle state machine: validation -> lock -> pre-check -> inject ->
// observe -> post-check -> evaluate -> report -> cleanup.
type Orchestrator struct {
	registry   *injection.Registry
	observer   observer.Observer
	reconciler *observer.ReconciliationChecker
	evaluator  *evaluator.Evaluator
	lock       safety.ExperimentLock
	knowledge  *model.OperatorKnowledge
	k8sClient  client.Client
	reportDir  string
	verbose    bool
	output     io.Writer
	logger     *slog.Logger
	depGraph   *model.DependencyGraph
}

// OrchestratorConfig holds configuration for creating an Orchestrator.
type OrchestratorConfig struct {
	Registry   *injection.Registry
	Observer   observer.Observer
	Reconciler *observer.ReconciliationChecker
	Evaluator  *evaluator.Evaluator
	Lock       safety.ExperimentLock
	Knowledge  *model.OperatorKnowledge
	K8sClient  client.Client
	ReportDir  string
	Verbose         bool
	DepGraph        *model.DependencyGraph
	KnowledgeModels []*model.OperatorKnowledge
	Logger          *slog.Logger
}

// ExperimentResult captures the outcome of running a chaos experiment.
type ExperimentResult struct {
	Experiment string                      `json:"experiment"`
	Phase      v1alpha1.ExperimentPhase    `json:"phase"`
	Verdict    v1alpha1.Verdict            `json:"verdict,omitempty"`
	Evaluation *evaluator.EvaluationResult `json:"evaluation,omitempty"`
	Report     *reporter.ExperimentReport  `json:"report,omitempty"`
	Error        string                      `json:"error,omitempty"`
	CleanupError string                      `json:"cleanupError,omitempty"`
}

// New creates a new Orchestrator with the given configuration.
func New(config OrchestratorConfig) *Orchestrator {
	output := io.Writer(os.Stdout)

	logger := config.Logger
	if logger == nil {
		if config.Verbose {
			logger = slog.New(slog.NewTextHandler(output, nil))
		} else {
			logger = slog.New(slog.NewTextHandler(io.Discard, nil))
		}
	}

	return &Orchestrator{
		registry:   config.Registry,
		observer:   config.Observer,
		reconciler: config.Reconciler,
		evaluator:  config.Evaluator,
		lock:       config.Lock,
		knowledge:  config.Knowledge,
		k8sClient:  config.K8sClient,
		reportDir:  config.ReportDir,
		verbose:    config.Verbose,
		output:     output,
		logger:     logger,
		depGraph:   config.DepGraph,
	}
}

// Run executes the full experiment lifecycle for the given ChaosExperiment.
// It proceeds through the phases: Pending -> SteadyStatePre -> Injecting ->
// Observing -> SteadyStatePost -> Evaluating -> Complete (or Aborted on error).
func (o *Orchestrator) Run(ctx context.Context, exp *v1alpha1.ChaosExperiment) (*ExperimentResult, error) {
	result := &ExperimentResult{
		Experiment: exp.Metadata.Name,
		Phase:      v1alpha1.PhasePending,
	}

	// 1. Validate
	o.logger.Info("phase transition", "phase", "PENDING", "experiment", exp.Metadata.Name, "action", "validating")

	// Determine namespace early (needed for blast radius validation)
	namespace := exp.Metadata.Namespace
	if namespace == "" {
		namespace = v1alpha1.DefaultNamespace
		o.logger.Warn("no namespace specified, using default", "namespace", namespace)
	}

	// Determine target resource for forbidden-resource validation
	targetResource := exp.Spec.Target.Resource
	if targetResource == "" {
		targetResource = fmt.Sprintf("%s/%s", exp.Spec.Target.Component, exp.Metadata.Name)
	}

	// Check blast radius
	if err := safety.ValidateBlastRadius(exp.Spec.BlastRadius, namespace, targetResource, exp.Spec.Injection.Count); err != nil {
		result.Error = fmt.Sprintf("blast radius validation failed: %v", err)
		result.Phase = v1alpha1.PhaseAborted
		return result, fmt.Errorf("blast radius: %w", err)
	}

	// Check danger level
	if err := safety.CheckDangerLevel(exp.Spec.Injection.DangerLevel, exp.Spec.BlastRadius.AllowDangerous); err != nil {
		result.Error = fmt.Sprintf("danger level check failed: %v", err)
		result.Phase = v1alpha1.PhaseAborted
		return result, fmt.Errorf("danger level: %w", err)
	}

	// Get injector
	injector, err := o.registry.Get(exp.Spec.Injection.Type)
	if err != nil {
		result.Error = fmt.Sprintf("unknown injection type: %v", err)
		result.Phase = v1alpha1.PhaseAborted
		return result, err
	}

	// Validate injection spec
	if err := injector.Validate(exp.Spec.Injection, exp.Spec.BlastRadius); err != nil {
		result.Error = fmt.Sprintf("injection validation failed: %v", err)
		result.Phase = v1alpha1.PhaseAborted
		return result, fmt.Errorf("injection validation: %w", err)
	}

	// Dry run check — skip lock acquisition since no faults are injected
	if exp.Spec.BlastRadius.DryRun {
		o.logger.Info("dry run", "injection", exp.Spec.Injection.Type, "operator", exp.Spec.Target.Operator, "component", exp.Spec.Target.Component)
		result.Phase = v1alpha1.PhaseComplete
		result.Verdict = v1alpha1.Inconclusive
		return result, nil
	}

	// Acquire experiment lock
	if err := o.lock.Acquire(ctx, exp.Spec.Target.Operator, exp.Metadata.Name); err != nil {
		result.Error = fmt.Sprintf("lock acquisition failed: %v", err)
		result.Phase = v1alpha1.PhaseAborted
		return result, fmt.Errorf("lock: %w", err)
	}
	defer o.lock.Release(exp.Spec.Target.Operator)

	// 2. Steady State Pre-Check
	o.logger.Info("phase transition", "phase", "STEADY_STATE_PRE", "action", "checking baseline")
	result.Phase = v1alpha1.PhaseSteadyStatePre

	var preCheck *v1alpha1.CheckResult
	if len(exp.Spec.SteadyState.Checks) > 0 {
		var preCheckErr error
		preCheck, preCheckErr = o.observer.CheckSteadyState(ctx, exp.Spec.SteadyState.Checks, namespace)
		if preCheckErr != nil {
			result.Error = fmt.Sprintf("steady state pre-check error: %v", preCheckErr)
			result.Phase = v1alpha1.PhaseAborted
			return result, fmt.Errorf("pre-check: %w", preCheckErr)
		}
	} else {
		preCheck = &v1alpha1.CheckResult{Passed: true, Timestamp: time.Now()}
	}

	if !preCheck.Passed {
		o.logger.Warn("pre-check failed", "reason", "system not in steady state")
		evalResult := o.evaluator.Evaluate(preCheck, preCheck, false, 0, 0, exp.Spec.Hypothesis)
		result.Evaluation = evalResult
		result.Verdict = evalResult.Verdict
		result.Phase = v1alpha1.PhaseAborted
		return result, fmt.Errorf("pre-check failed: system not in steady state")
	}

	// 3. Inject
	o.logger.Info("phase transition", "phase", "INJECTING", "injection", exp.Spec.Injection.Type)
	result.Phase = v1alpha1.PhaseInjecting

	cleanup, events, err := injector.Inject(ctx, exp.Spec.Injection, namespace)
	if err != nil {
		result.Error = fmt.Sprintf("injection failed: %v", err)
		result.Phase = v1alpha1.PhaseAborted
		return result, fmt.Errorf("injection: %w", err)
	}

	// Ensure cleanup runs even if the parent context is cancelled
	defer func() {
		if cleanup != nil {
			o.logger.Info("phase transition", "phase", "CLEANUP")
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), defaultCleanupTimeout)
			defer cleanupCancel()
			if cleanErr := cleanup(cleanupCtx); cleanErr != nil {
				o.logger.Warn("cleanup warning", "error", cleanErr)
				result.CleanupError = cleanErr.Error()
				if result.Report != nil {
					result.Report.CleanupError = cleanErr.Error()
				}
			}
		}
	}()

	o.logger.Info("injection complete", "events", len(events))

	// 4. Observe — two-phase blackboard pattern
	o.logger.Info("phase transition", "phase", "OBSERVING", "action", "waiting for recovery")
	result.Phase = v1alpha1.PhaseObserving

	recoveryTimeout := exp.Spec.Hypothesis.RecoveryTimeout.Duration
	if recoveryTimeout == 0 {
		recoveryTimeout = defaultRecoveryTimeout
	}

	board := observer.NewObservationBoard()

	// Phase 1: Reconciliation (blocking)
	if o.knowledge != nil {
		component := o.knowledge.GetComponent(exp.Spec.Target.Component)
		if component != nil && o.reconciler != nil {
			reconContributor := observer.NewReconciliationContributor(o.reconciler, component, namespace, recoveryTimeout)
			if reconErr := reconContributor.Observe(ctx, board); reconErr != nil {
				o.logger.Warn("reconciliation contributor error", "error", reconErr)
			}
		}
	}

	// 5. Steady State Post-Check + Collateral — Phase 2 (concurrent)
	o.logger.Info("phase transition", "phase", "STEADY_STATE_POST", "action", "verifying recovery")
	result.Phase = v1alpha1.PhaseSteadyStatePost

	var phase2Contributors []observer.ObservationContributor

	if len(exp.Spec.SteadyState.Checks) > 0 {
		phase2Contributors = append(phase2Contributors, observer.NewSteadyStateContributor(
			o.observer, exp.Spec.SteadyState.Checks, namespace))
	} else {
		// No checks defined — write a "passed" finding to preserve existing behavior
		board.AddFinding(observer.Finding{
			Source: observer.SourceSteadyState,
			Passed: true,
			Checks: &v1alpha1.CheckResult{Passed: true, Timestamp: time.Now()},
		})
	}

	if o.depGraph != nil {
		ref := model.ComponentRef{
			Operator:  exp.Spec.Target.Operator,
			Component: exp.Spec.Target.Component,
		}
		dependents := o.depGraph.DirectDependents(ref)
		if len(dependents) > 0 {
			phase2Contributors = append(phase2Contributors, observer.NewCollateralContributor(
				o.observer, dependents))
		}
	}

	if len(phase2Contributors) > 0 {
		if errs := observer.RunContributors(ctx, board, phase2Contributors); len(errs) > 0 {
			for _, e := range errs {
				o.logger.Warn("phase 2 contributor error", "error", e)
			}
		}
	}

	// 6. Evaluate
	o.logger.Info("phase transition", "phase", "EVALUATING")
	result.Phase = v1alpha1.PhaseEvaluating

	evalResult := o.evaluator.EvaluateFromFindings(board.Findings(), exp.Spec.Hypothesis)
	result.Evaluation = evalResult
	result.Verdict = evalResult.Verdict

	// Extract data from board for report
	var reconciliationResult *observer.ReconciliationResult
	for _, f := range board.FindingsBySource(observer.SourceReconciliation) {
		reconciliationResult = f.ReconciliationResult
	}

	var postCheck *v1alpha1.CheckResult
	for _, f := range board.FindingsBySource(observer.SourceSteadyState) {
		postCheck = f.Checks
	}
	if postCheck == nil {
		postCheck = &v1alpha1.CheckResult{Passed: true, Timestamp: time.Now()}
	}

	// 7. Report

	// Extract injection targets from events
	var injectionTargets []string
	for _, ev := range events {
		if ev.Target != "" {
			injectionTargets = append(injectionTargets, ev.Target)
		}
	}

	// Build collateral findings for report
	var collateralFindings []reporter.CollateralFinding
	for _, f := range board.FindingsBySource(observer.SourceCollateral) {
		collateralFindings = append(collateralFindings, reporter.CollateralFinding{
			Operator:  f.Operator,
			Component: f.Component,
			Passed:    f.Passed,
			Checks:    f.Checks,
		})
	}

	report := reporter.ExperimentReport{
		Experiment: exp.Metadata.Name,
		Timestamp:  time.Now(),
		Target: reporter.TargetReport{
			Operator:  exp.Spec.Target.Operator,
			Component: exp.Spec.Target.Component,
			Resource:  exp.Spec.Target.Resource,
		},
		Injection: reporter.InjectionReport{
			Type:      string(exp.Spec.Injection.Type),
			Targets:   injectionTargets,
			Timestamp: time.Now(),
		},
		SteadyState: reporter.SteadyStateReport{
			Pre:  preCheck,
			Post: postCheck,
		},
		Evaluation:     *evalResult,
		Reconciliation: reconciliationResult,
		Collateral:     collateralFindings,
	}
	result.Report = &report

	// Write JSON report if reportDir specified
	if o.reportDir != "" {
		reportPath := filepath.Join(o.reportDir, fmt.Sprintf("%s-%s.json", exp.Metadata.Name, time.Now().Format("20060102-150405")))
		r, err := reporter.NewJSONFileReporter(reportPath)
		if err != nil {
			o.logger.Warn("creating report file", "path", reportPath, "error", err)
		} else {
			if writeErr := r.Write(report); writeErr != nil {
				o.logger.Warn("writing report", "path", reportPath, "error", writeErr)
			}
			if closeErr := r.Close(); closeErr != nil {
				o.logger.Warn("closing report file", "path", reportPath, "error", closeErr)
			}
		}
	}

	// Store result as ConfigMap in cluster
	if o.k8sClient != nil {
		o.storeResultConfigMap(ctx, exp, namespace, report)
	}

	o.logger.Info("phase transition", "phase", "COMPLETE", "verdict", evalResult.Verdict)
	result.Phase = v1alpha1.PhaseComplete

	return result, nil
}

// configMapNameMaxLen is the maximum length for a Kubernetes resource name.
const configMapNameMaxLen = 253

// labelValueMaxLen is the maximum length for a Kubernetes label value.
const labelValueMaxLen = 63

// truncateLabel truncates a string to the maximum Kubernetes label value length.
func truncateLabel(s string) string {
	if len(s) <= labelValueMaxLen {
		return s
	}
	return s[:labelValueMaxLen]
}

// storeResultConfigMap creates a ConfigMap in the experiment's namespace
// containing the JSON-serialized ExperimentReport, making results visible
// via kubectl get configmap -l app.kubernetes.io/managed-by=odh-chaos.
func (o *Orchestrator) storeResultConfigMap(ctx context.Context, exp *v1alpha1.ChaosExperiment, namespace string, report reporter.ExperimentReport) {
	// Use a dedicated context so that ConfigMap storage succeeds even if the
	// parent context is near its deadline.
	storeCtx, storeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer storeCancel()

	reportJSON, err := json.Marshal(report)
	if err != nil {
		o.logger.Warn("marshaling report for ConfigMap", "error", err)
		return
	}

	cmName := "chaos-result-" + exp.Metadata.Name
	if len(cmName) > configMapNameMaxLen {
		cmName = cmName[:configMapNameMaxLen]
	}
	// Ensure the name does not end with a non-alphanumeric character
	cmName = strings.TrimRight(cmName, "-._")

	cmLabels := map[string]string{
		"app.kubernetes.io/managed-by":    "odh-chaos",
		"chaos.opendatahub.io/experiment": truncateLabel(exp.Metadata.Name),
		"chaos.opendatahub.io/verdict":    strings.ToLower(string(report.Evaluation.Verdict)),
	}
	cmAnnotations := map[string]string{
		"chaos.opendatahub.io/timestamp": report.Timestamp.UTC().Format(time.RFC3339),
	}
	cmData := map[string]string{
		"result.json": string(reportJSON),
	}

	existing := &corev1.ConfigMap{}
	existing.Name = cmName
	existing.Namespace = namespace
	result, err := controllerutil.CreateOrUpdate(storeCtx, o.k8sClient, existing, func() error {
		existing.Labels = cmLabels
		existing.Annotations = cmAnnotations
		existing.Data = cmData
		return nil
	})
	if err != nil {
		o.logger.Warn("storing result ConfigMap", "name", cmName, "namespace", namespace, "error", err)
	} else {
		o.logger.Info("result ConfigMap stored", "name", cmName, "namespace", namespace, "operation", result)
	}
}

