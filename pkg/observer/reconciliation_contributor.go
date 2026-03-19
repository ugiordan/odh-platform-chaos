package observer

import (
	"context"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
)

type ReconciliationCheckerInterface interface {
	CheckReconciliation(ctx context.Context, component *model.ComponentModel, namespace string, timeout time.Duration) (*ReconciliationResult, error)
}

type ReconciliationContributor struct {
	checker   ReconciliationCheckerInterface
	component *model.ComponentModel
	namespace string
	timeout   time.Duration
}

func NewReconciliationContributor(checker ReconciliationCheckerInterface, component *model.ComponentModel, namespace string, timeout time.Duration) *ReconciliationContributor {
	return &ReconciliationContributor{
		checker:   checker,
		component: component,
		namespace: namespace,
		timeout:   timeout,
	}
}

func (c *ReconciliationContributor) Observe(ctx context.Context, board *ObservationBoard) error {
	result, err := c.checker.CheckReconciliation(ctx, c.component, c.namespace, c.timeout)
	if err != nil {
		board.AddFinding(Finding{
			Source:    SourceReconciliation,
			Component: c.component.Name,
			Passed:   false,
			Details:  err.Error(),
		})
		return err
	}

	board.AddFinding(Finding{
		Source:               SourceReconciliation,
		Component:            c.component.Name,
		Passed:               result.AllReconciled,
		ReconciliationResult: result,
	})
	return nil
}
