package safety

import (
	"context"
	"fmt"
	"sync"
)

type ExperimentLock interface {
	Acquire(ctx context.Context, operator string, experimentName string) error
	Release(operator string)
}

type localExperimentLock struct {
	mu    sync.Mutex
	locks map[string]string // operator -> experimentName
}

func NewLocalExperimentLock() ExperimentLock {
	return &localExperimentLock{
		locks: make(map[string]string),
	}
}

func (l *localExperimentLock) Acquire(_ context.Context, operator string, experimentName string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if existing, ok := l.locks[operator]; ok {
		return fmt.Errorf("operator %q already has active experiment %q", operator, existing)
	}

	l.locks[operator] = experimentName
	return nil
}

func (l *localExperimentLock) Release(operator string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.locks, operator)
}
