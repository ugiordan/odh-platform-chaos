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
)

// CRDMutationInjector injects faults by mutating spec fields on custom resources.
type CRDMutationInjector struct {
	client client.Client
}

// NewCRDMutationInjector creates a new CRDMutationInjector using the given Kubernetes client.
func NewCRDMutationInjector(c client.Client) *CRDMutationInjector {
	return &CRDMutationInjector{client: c}
}

// Validate checks that the injection spec contains the required parameters: apiVersion, kind, name, field, and value.
func (m *CRDMutationInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	if _, ok := spec.Parameters["apiVersion"]; !ok {
		return fmt.Errorf("CRDMutation requires 'apiVersion' parameter")
	}
	if _, ok := spec.Parameters["kind"]; !ok {
		return fmt.Errorf("CRDMutation requires 'kind' parameter")
	}
	if _, ok := spec.Parameters["name"]; !ok {
		return fmt.Errorf("CRDMutation requires 'name' parameter")
	}
	if err := validateK8sName("name", spec.Parameters["name"]); err != nil {
		return err
	}
	if _, ok := spec.Parameters["field"]; !ok {
		return fmt.Errorf("CRDMutation requires 'field' parameter (JSON path to mutate)")
	}
	if err := validateFieldName("field", spec.Parameters["field"]); err != nil {
		return err
	}
	if _, ok := spec.Parameters["value"]; !ok {
		return fmt.Errorf("CRDMutation requires 'value' parameter (JSON value to set)")
	}
	return nil
}

// Inject mutates a spec field on the target custom resource and returns a cleanup function that restores the original value.
func (m *CRDMutationInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(spec.Parameters["apiVersion"])
	obj.SetKind(spec.Parameters["kind"])

	key := types.NamespacedName{
		Name:      spec.Parameters["name"],
		Namespace: namespace,
	}

	if err := m.client.Get(ctx, key, obj); err != nil {
		return nil, nil, fmt.Errorf("getting resource %s/%s: %w", spec.Parameters["kind"], spec.Parameters["name"], err)
	}

	// Save original field value for cleanup
	fieldName := spec.Parameters["field"]
	specMap, _, _ := unstructured.NestedMap(obj.Object, "spec")
	var originalValue interface{}
	if specMap != nil {
		originalValue = specMap[fieldName]
	}

	// Build rollback data for crash-safe recovery
	rollbackInfo := map[string]interface{}{
		"apiVersion":    spec.Parameters["apiVersion"],
		"kind":          spec.Parameters["kind"],
		"field":         fieldName,
		"originalValue": originalValue,
	}
	rollbackStr, err := safety.WrapRollbackData(rollbackInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing rollback data for %s/%s: %w", spec.Parameters["kind"], spec.Parameters["name"], err)
	}

	// Build annotations map with rollback data
	annotationsMap := map[string]interface{}{
		safety.RollbackAnnotationKey: rollbackStr,
	}

	// Build labels map with chaos labels
	chaosLabels := safety.ChaosLabels(string(v1alpha1.CRDMutation))
	labelsMap := make(map[string]interface{}, len(chaosLabels))
	for k, v := range chaosLabels {
		labelsMap[k] = v
	}

	// Apply mutation via merge patch including rollback annotation and chaos labels
	patchMap := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotationsMap,
			"labels":      labelsMap,
		},
		"spec": map[string]interface{}{
			fieldName: spec.Parameters["value"],
		},
	}
	patch, err := json.Marshal(patchMap)
	if err != nil {
		return nil, nil, fmt.Errorf("building mutation patch: %w", err)
	}
	if err := m.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return nil, nil, fmt.Errorf("applying mutation: %w", err)
	}

	// Save GVK info for cleanup re-fetch
	apiVersion := spec.Parameters["apiVersion"]
	kind := spec.Parameters["kind"]

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.CRDMutation, key.String(), "mutated",
			map[string]string{
				"field": fieldName,
				"value": spec.Parameters["value"],
			}),
	}

	// Cleanup restores original field value and removes rollback metadata
	cleanup := func(ctx context.Context) error {
		// Re-fetch the resource to get current state as patch target
		current := &unstructured.Unstructured{}
		current.SetAPIVersion(apiVersion)
		current.SetKind(kind)
		if err := m.client.Get(ctx, key, current); err != nil {
			return fmt.Errorf("re-fetching resource for cleanup: %w", err)
		}

		// Build a merge patch that restores the mutated field and removes
		// the rollback annotation and chaos labels. In merge patch, setting
		// a key to null removes it.
		restoreAnnotations := map[string]interface{}{
			safety.RollbackAnnotationKey: nil,
		}
		restoreLabels := make(map[string]interface{})
		for k := range chaosLabels {
			restoreLabels[k] = nil
		}

		// When originalValue is nil, JSON merge patch serializes it as null,
		// which removes the key -- exactly the desired behavior.
		restorePatchMap := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": restoreAnnotations,
				"labels":      restoreLabels,
			},
			"spec": map[string]interface{}{
				fieldName: originalValue,
			},
		}
		restorePatch, err := json.Marshal(restorePatchMap)
		if err != nil {
			return fmt.Errorf("building restore patch: %w", err)
		}

		return m.client.Patch(ctx, current, client.RawPatch(types.MergePatchType, restorePatch))
	}

	return cleanup, events, nil
}
