package observer

import (
	"sync"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
)

type FindingSource string

const (
	SourceReconciliation FindingSource = "reconciliation"
	SourceSteadyState    FindingSource = "steady_state"
	SourceCollateral     FindingSource = "collateral"
)

type Finding struct {
	Source               FindingSource
	Component            string
	Operator             string
	Passed               bool
	Details              string
	Checks               *v1alpha1.CheckResult
	ReconciliationResult *ReconciliationResult
}

type ObservationBoard struct {
	mu       sync.Mutex
	findings []Finding
}

func NewObservationBoard() *ObservationBoard {
	return &ObservationBoard{}
}

func (b *ObservationBoard) AddFinding(f Finding) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.findings = append(b.findings, f)
}

func (b *ObservationBoard) Findings() []Finding {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Finding(nil), b.findings...)
}

func (b *ObservationBoard) FindingsBySource(source FindingSource) []Finding {
	b.mu.Lock()
	defer b.mu.Unlock()
	var result []Finding
	for _, f := range b.findings {
		if f.Source == source {
			result = append(result, f)
		}
	}
	return result
}
