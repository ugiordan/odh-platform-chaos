package reporter

import (
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/evaluator"
	"github.com/stretchr/testify/assert"
)

func TestComputeSummary(t *testing.T) {
	reports := []ExperimentReport{
		{Experiment: "a", Evaluation: evaluator.EvaluationResult{Verdict: v1alpha1.Resilient, RecoveryTime: 2 * time.Second}},
		{Experiment: "b", Evaluation: evaluator.EvaluationResult{Verdict: v1alpha1.Resilient, RecoveryTime: 3 * time.Second}},
		{Experiment: "c", Evaluation: evaluator.EvaluationResult{Verdict: v1alpha1.Degraded, RecoveryTime: 10 * time.Second}},
		{Experiment: "d", Evaluation: evaluator.EvaluationResult{Verdict: v1alpha1.Failed, RecoveryTime: 0}},
		{Experiment: "e", Evaluation: evaluator.EvaluationResult{Verdict: v1alpha1.Inconclusive, RecoveryTime: 0}},
	}

	s := ComputeSummary(reports)
	assert.Equal(t, 5, s.Total)
	assert.Equal(t, 2, s.Resilient)
	assert.Equal(t, 1, s.Degraded)
	assert.Equal(t, 1, s.Failed)
	assert.Equal(t, 1, s.Inconclusive)
	assert.InDelta(t, 0.40, s.PassRate, 0.001)
	assert.Equal(t, 15*time.Second, s.TotalTime)
}

func TestComputeSummaryEmpty(t *testing.T) {
	s := ComputeSummary(nil)
	assert.Equal(t, 0, s.Total)
	assert.Equal(t, float64(0), s.PassRate)
}
