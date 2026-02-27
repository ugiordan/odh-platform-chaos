package injection

import (
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CRDMutationInjector struct {
	client client.Client
}

func NewCRDMutationInjector(c client.Client) *CRDMutationInjector {
	return &CRDMutationInjector{client: c}
}

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
	if _, ok := spec.Parameters["field"]; !ok {
		return fmt.Errorf("CRDMutation requires 'field' parameter (JSON path to mutate)")
	}
	if _, ok := spec.Parameters["value"]; !ok {
		return fmt.Errorf("CRDMutation requires 'value' parameter (JSON value to set)")
	}
	return nil
}

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

	// Save original for cleanup
	originalData, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, nil, fmt.Errorf("saving original state: %w", err)
	}

	// Apply mutation via merge patch (use json.Marshal for safe serialization)
	patchMap := map[string]interface{}{
		"spec": map[string]interface{}{
			spec.Parameters["field"]: spec.Parameters["value"],
		},
	}
	patch, err := json.Marshal(patchMap)
	if err != nil {
		return nil, nil, fmt.Errorf("building mutation patch: %w", err)
	}
	if err := m.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return nil, nil, fmt.Errorf("applying mutation: %w", err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.CRDMutation, key.String(), "mutated",
			map[string]string{
				"field": spec.Parameters["field"],
				"value": spec.Parameters["value"],
			}),
	}

	// Cleanup restores original state
	cleanup := func(ctx context.Context) error {
		var original map[string]interface{}
		if err := json.Unmarshal(originalData, &original); err != nil {
			return fmt.Errorf("unmarshaling original state: %w", err)
		}
		restored := &unstructured.Unstructured{Object: original}
		return m.client.Update(ctx, restored)
	}

	return cleanup, events, nil
}
