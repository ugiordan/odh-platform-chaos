package observer

import (
	"context"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
)

type CollateralContributor struct {
	observer   Observer
	dependents []*model.ResolvedComponent
}

func NewCollateralContributor(obs Observer, dependents []*model.ResolvedComponent) *CollateralContributor {
	return &CollateralContributor{
		observer:   obs,
		dependents: dependents,
	}
}

func (c *CollateralContributor) Observe(ctx context.Context, board *ObservationBoard) error {
	for _, dep := range c.dependents {
		if len(dep.Component.SteadyState.Checks) == 0 {
			continue
		}

		result, err := c.observer.CheckSteadyState(ctx, dep.Component.SteadyState.Checks, dep.Namespace)
		if err != nil {
			board.AddFinding(Finding{
				Source:    SourceCollateral,
				Component: dep.Component.Name,
				Operator:  dep.Ref.Operator,
				Passed:    false,
				Details:   err.Error(),
			})
			continue
		}

		board.AddFinding(Finding{
			Source:    SourceCollateral,
			Component: dep.Component.Name,
			Operator:  dep.Ref.Operator,
			Passed:    result.Passed,
			Checks:    result,
		})
	}
	return nil
}
