package observer

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObservationBoard_AddAndRetrieve(t *testing.T) {
	board := NewObservationBoard()

	board.AddFinding(Finding{
		Source:    SourceReconciliation,
		Component: "comp-a",
		Passed:    true,
		Details:   "all reconciled",
	})
	board.AddFinding(Finding{
		Source:    SourceSteadyState,
		Component: "comp-a",
		Passed:    true,
	})
	board.AddFinding(Finding{
		Source:    SourceCollateral,
		Component: "comp-b",
		Operator:  "other-op",
		Passed:    false,
		Details:   "check failed",
	})

	findings := board.Findings()
	require.Len(t, findings, 3)
	assert.Equal(t, SourceReconciliation, findings[0].Source)
	assert.Equal(t, SourceCollateral, findings[2].Source)
}

func TestObservationBoard_FindingsBySource(t *testing.T) {
	board := NewObservationBoard()
	board.AddFinding(Finding{Source: SourceReconciliation, Passed: true})
	board.AddFinding(Finding{Source: SourceSteadyState, Passed: true})
	board.AddFinding(Finding{Source: SourceCollateral, Passed: false})
	board.AddFinding(Finding{Source: SourceCollateral, Passed: true})

	collateral := board.FindingsBySource(SourceCollateral)
	require.Len(t, collateral, 2)

	recon := board.FindingsBySource(SourceReconciliation)
	require.Len(t, recon, 1)
}

func TestObservationBoard_FindingsReturnsCopy(t *testing.T) {
	board := NewObservationBoard()
	board.AddFinding(Finding{Source: SourceSteadyState, Passed: true})

	findings := board.Findings()
	findings[0].Passed = false

	original := board.Findings()
	assert.True(t, original[0].Passed, "original should not be mutated")
}

func TestObservationBoard_ConcurrentWrites(t *testing.T) {
	board := NewObservationBoard()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			board.AddFinding(Finding{
				Source:    SourceCollateral,
				Component: "comp",
				Passed:    idx%2 == 0,
			})
		}(i)
	}
	wg.Wait()

	assert.Len(t, board.Findings(), 100)
}
