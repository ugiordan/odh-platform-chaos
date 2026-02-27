package injection

import (
	"context"
	"fmt"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
)

// CleanupFunc is a function that reverses a fault injection when called.
type CleanupFunc func(ctx context.Context) error

// Injector defines the interface for fault injection strategies.
type Injector interface {
	Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error
	Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error)
}

// Registry maps injection types to their corresponding Injector implementations.
type Registry struct {
	injectors map[v1alpha1.InjectionType]Injector
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		injectors: make(map[v1alpha1.InjectionType]Injector),
	}
}

// Register adds an Injector for the given injection type.
func (r *Registry) Register(t v1alpha1.InjectionType, i Injector) {
	r.injectors[t] = i
}

// Get returns the Injector registered for the given type, or an error if none is found.
func (r *Registry) Get(t v1alpha1.InjectionType) (Injector, error) {
	if inj, ok := r.injectors[t]; ok {
		return inj, nil
	}
	return nil, fmt.Errorf("unknown injection type %q; registered types: %v", t, r.ListTypes())
}

// ListTypes returns all registered injection types.
func (r *Registry) ListTypes() []v1alpha1.InjectionType {
	types := make([]v1alpha1.InjectionType, 0, len(r.injectors))
	for t := range r.injectors {
		types = append(types, t)
	}
	return types
}

// NewEvent creates an InjectionEvent with the current timestamp.
func NewEvent(t v1alpha1.InjectionType, target string, action string, details map[string]string) v1alpha1.InjectionEvent {
	return v1alpha1.InjectionEvent{
		Timestamp: time.Now(),
		Type:      t,
		Target:    target,
		Action:    action,
		Details:   details,
	}
}
