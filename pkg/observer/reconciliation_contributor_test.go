package observer

import (
	"context"
	"testing"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockReconciliationChecker struct {
	result *ReconciliationResult
	err    error
}

func (m *mockReconciliationChecker) CheckReconciliation(
	ctx context.Context,
	component *model.ComponentModel,
	namespace string,
	timeout time.Duration,
) (*ReconciliationResult, error) {
	return m.result, m.err
}

func TestReconciliationContributor_WritesFindings(t *testing.T) {
	board := NewObservationBoard()
	checker := &mockReconciliationChecker{
		result: &ReconciliationResult{
			AllReconciled:   true,
			ReconcileCycles: 3,
			RecoveryTime:    10 * time.Second,
		},
	}
	comp := &model.ComponentModel{Name: "test-comp"}

	contrib := NewReconciliationContributor(checker, comp, "test-ns", 60*time.Second)
	err := contrib.Observe(context.Background(), board)
	require.NoError(t, err)

	findings := board.FindingsBySource(SourceReconciliation)
	require.Len(t, findings, 1)
	assert.True(t, findings[0].Passed)
	assert.Equal(t, "test-comp", findings[0].Component)
	assert.NotNil(t, findings[0].ReconciliationResult)
	assert.Equal(t, 3, findings[0].ReconciliationResult.ReconcileCycles)
}

func TestReconciliationContributor_NotReconciled(t *testing.T) {
	board := NewObservationBoard()
	checker := &mockReconciliationChecker{
		result: &ReconciliationResult{AllReconciled: false, ReconcileCycles: 5},
	}
	comp := &model.ComponentModel{Name: "failing-comp"}

	contrib := NewReconciliationContributor(checker, comp, "ns", 30*time.Second)
	err := contrib.Observe(context.Background(), board)
	require.NoError(t, err)

	findings := board.FindingsBySource(SourceReconciliation)
	require.Len(t, findings, 1)
	assert.False(t, findings[0].Passed)
}
