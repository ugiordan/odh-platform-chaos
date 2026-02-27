package evaluator

import (
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
)

// EvaluationResult contains the outcome of evaluating a chaos experiment.
type EvaluationResult struct {
	Verdict         v1alpha1.Verdict `json:"verdict"`
	Confidence      string           `json:"confidence"`
	RecoveryTime    time.Duration    `json:"recoveryTime"`
	ReconcileCycles int              `json:"reconcileCycles"`
	Deviations      []Deviation      `json:"deviations,omitempty"`
}

// Deviation records a deviation from expected behavior during an experiment.
type Deviation struct {
	Type   string `json:"type"`
	Detail string `json:"detail"`
}
