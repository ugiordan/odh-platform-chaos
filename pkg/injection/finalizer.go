package injection

import (
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/safety"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const defaultFinalizerName = "chaos.opendatahub.io/block"

// FinalizerBlockInjector adds a stuck finalizer to a resource, blocking its
// deletion and testing how operators handle resources stuck in a Terminating state.
type FinalizerBlockInjector struct {
	client client.Client
}

// NewFinalizerBlockInjector creates a new FinalizerBlockInjector.
func NewFinalizerBlockInjector(c client.Client) *FinalizerBlockInjector {
	return &FinalizerBlockInjector{client: c}
}

// Validate checks that the injection spec contains the required parameters:
//   - name: the name of the target resource
//   - kind: the kind of the target resource
//
// If "finalizer" is missing, the default "chaos.opendatahub.io/block" is used.
// If "apiVersion" is missing, it defaults to "v1".
func (f *FinalizerBlockInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	if _, ok := spec.Parameters["kind"]; !ok {
		return fmt.Errorf("FinalizerBlock requires 'kind' parameter")
	}

	if _, ok := spec.Parameters["name"]; !ok {
		return fmt.Errorf("FinalizerBlock requires 'name' parameter")
	}

	return nil
}

// Inject adds a finalizer to the target resource:
//  1. Creates an Unstructured object with the apiVersion/kind from parameters
//  2. Fetches the resource from the cluster
//  3. Adds the finalizer to its finalizers list
//  4. Updates the object
//  5. Returns a cleanup function that removes the finalizer
func (f *FinalizerBlockInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	apiVersion := spec.Parameters["apiVersion"]
	if apiVersion == "" {
		apiVersion = "v1"
	}

	kind := spec.Parameters["kind"]
	name := spec.Parameters["name"]

	finalizerName := spec.Parameters["finalizer"]
	if finalizerName == "" {
		finalizerName = defaultFinalizerName
	}

	// Build unstructured object to fetch the target resource
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)

	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	if err := f.client.Get(ctx, key, obj); err != nil {
		return nil, nil, fmt.Errorf("getting resource %s/%s: %w", kind, name, err)
	}

	// Add the finalizer using controller-runtime helper
	if controllerutil.AddFinalizer(obj, finalizerName) {
		// Store rollback annotation for crash-safe recovery
		rollbackData := map[string]string{
			"finalizer": finalizerName,
		}
		rollbackJSON, err := json.Marshal(rollbackData)
		if err != nil {
			return nil, nil, fmt.Errorf("serializing rollback data for %s/%s: %w", kind, name, err)
		}

		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[safety.RollbackAnnotationKey] = string(rollbackJSON)
		obj.SetAnnotations(annotations)

		// Add chaos labels
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		for k, v := range safety.ChaosLabels(string(v1alpha1.FinalizerBlock)) {
			labels[k] = v
		}
		obj.SetLabels(labels)

		if err := f.client.Update(ctx, obj); err != nil {
			return nil, nil, fmt.Errorf("adding finalizer to %s/%s: %w", kind, name, err)
		}
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.FinalizerBlock, key.String(), "addFinalizer",
			map[string]string{
				"apiVersion": apiVersion,
				"kind":       kind,
				"name":       name,
				"finalizer":  finalizerName,
			}),
	}

	// Cleanup removes the finalizer, rollback annotation, and chaos labels
	cleanup := func(ctx context.Context) error {
		current := &unstructured.Unstructured{}
		current.SetAPIVersion(apiVersion)
		current.SetKind(kind)

		if err := f.client.Get(ctx, key, current); err != nil {
			return fmt.Errorf("re-fetching %s/%s for cleanup: %w", kind, name, err)
		}

		changed := controllerutil.RemoveFinalizer(current, finalizerName)

		// Remove rollback annotation
		annotations := current.GetAnnotations()
		if annotations != nil {
			if _, ok := annotations[safety.RollbackAnnotationKey]; ok {
				delete(annotations, safety.RollbackAnnotationKey)
				current.SetAnnotations(annotations)
				changed = true
			}
		}

		// Remove chaos labels
		labels := current.GetLabels()
		if labels != nil {
			for k := range safety.ChaosLabels(string(v1alpha1.FinalizerBlock)) {
				if _, ok := labels[k]; ok {
					delete(labels, k)
					changed = true
				}
			}
			current.SetLabels(labels)
		}

		if changed {
			if err := f.client.Update(ctx, current); err != nil {
				return fmt.Errorf("removing finalizer from %s/%s: %w", kind, name, err)
			}
		}

		return nil
	}

	return cleanup, events, nil
}
