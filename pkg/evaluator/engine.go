package evaluator

import (
	"fmt"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
)

// Evaluator classifies experiment outcomes into verdicts based on
// steady-state checks, reconciliation status, recovery time, and
// reconcile cycle counts.
type Evaluator struct {
	maxReconcileCycles int
}

// New creates an Evaluator that flags excessive reconciliation when cycles exceed maxReconcileCycles.
func New(maxReconcileCycles int) *Evaluator {
	return &Evaluator{maxReconcileCycles: maxReconcileCycles}
}

// Evaluate compares pre- and post-injection check results against the hypothesis and returns a verdict.
func (e *Evaluator) Evaluate(
	preCheck *v1alpha1.CheckResult,
	postCheck *v1alpha1.CheckResult,
	allReconciled bool,
	reconcileCycles int,
	recoveryTime time.Duration,
	hypothesis v1alpha1.HypothesisSpec,
) *EvaluationResult {
	result := &EvaluationResult{
		RecoveryTime:    recoveryTime,
		ReconcileCycles: reconcileCycles,
	}

	// 1. Baseline not established
	if !preCheck.Passed {
		result.Verdict = v1alpha1.Inconclusive
		result.Confidence = fmt.Sprintf(
			"pre-check failed: %d/%d checks passed",
			preCheck.ChecksPassed, preCheck.ChecksRun)
		return result
	}

	// 2. Did it recover?
	if postCheck.Passed && allReconciled {
		result.Verdict = v1alpha1.Resilient
	} else if postCheck.Passed && !allReconciled {
		result.Verdict = v1alpha1.Degraded
		result.Deviations = append(result.Deviations, Deviation{
			Type:   "partial_reconciliation",
			Detail: "steady state checks passed but not all resources reconciled",
		})
	} else {
		result.Verdict = v1alpha1.Failed
	}

	// 3. Recovery time
	if recoveryTime > hypothesis.RecoveryTimeout.Duration {
		if result.Verdict == v1alpha1.Resilient {
			result.Verdict = v1alpha1.Degraded
		}
		result.Deviations = append(result.Deviations, Deviation{
			Type: "slow_recovery",
			Detail: fmt.Sprintf("recovered in %s, expected within %s",
				recoveryTime, hypothesis.RecoveryTimeout.Duration),
		})
	}

	// 4. Excessive reconcile cycles
	if e.maxReconcileCycles > 0 && reconcileCycles > e.maxReconcileCycles {
		if result.Verdict == v1alpha1.Resilient {
			result.Verdict = v1alpha1.Degraded
		}
		result.Deviations = append(result.Deviations, Deviation{
			Type: "excessive_reconciliation",
			Detail: fmt.Sprintf("%d cycles (max %d)",
				reconcileCycles, e.maxReconcileCycles),
		})
	}

	// 5. Confidence qualifier
	result.Confidence = fmt.Sprintf(
		"%d/%d steady-state checks passed, %s recovery, %d reconcile cycles",
		postCheck.ChecksPassed, postCheck.ChecksRun,
		recoveryTime, reconcileCycles)

	return result
}
