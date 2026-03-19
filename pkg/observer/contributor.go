package observer

import (
	"context"
	"sync"
)

type ObservationContributor interface {
	Observe(ctx context.Context, board *ObservationBoard) error
}

// RunContributors spawns a goroutine for each contributor using sync.WaitGroup,
// waits for all to complete, and returns all errors. One contributor's failure
// does NOT cancel siblings. Do NOT use errgroup.WithContext.
func RunContributors(ctx context.Context, board *ObservationBoard, contributors []ObservationContributor) []error {
	if len(contributors) == 0 {
		return nil
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, c := range contributors {
		wg.Add(1)
		go func(contrib ObservationContributor) {
			defer wg.Done()
			if err := contrib.Observe(ctx, board); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(c)
	}

	wg.Wait()
	return errs
}
