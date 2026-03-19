package observer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testContributor struct {
	finding Finding
	err     error
	delay   time.Duration
	called  atomic.Bool
}

func (c *testContributor) Observe(ctx context.Context, board *ObservationBoard) error {
	c.called.Store(true)
	if c.delay > 0 {
		select {
		case <-time.After(c.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	board.AddFinding(c.finding)
	return c.err
}

func TestRunContributors_AllSucceed(t *testing.T) {
	board := NewObservationBoard()
	contributors := []ObservationContributor{
		&testContributor{finding: Finding{Source: SourceSteadyState, Passed: true}},
		&testContributor{finding: Finding{Source: SourceCollateral, Passed: true}},
	}

	errs := RunContributors(context.Background(), board, contributors)
	assert.Empty(t, errs)
	assert.Len(t, board.Findings(), 2)
}

func TestRunContributors_OneFailsOthersComplete(t *testing.T) {
	board := NewObservationBoard()
	contributors := []ObservationContributor{
		&testContributor{
			finding: Finding{Source: SourceSteadyState, Passed: true},
			err:     errors.New("observer error"),
		},
		&testContributor{
			finding: Finding{Source: SourceCollateral, Passed: false},
			delay:   10 * time.Millisecond,
		},
	}

	errs := RunContributors(context.Background(), board, contributors)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "observer error")
	assert.Len(t, board.Findings(), 2)
}

func TestRunContributors_Empty(t *testing.T) {
	board := NewObservationBoard()
	errs := RunContributors(context.Background(), board, nil)
	assert.Empty(t, errs)
	assert.Empty(t, board.Findings())
}
