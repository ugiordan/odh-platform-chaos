package injection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func (m *CRDMutationInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateCRDMutationParams(spec)
}

// Inject mutates a field on the target custom resource and returns a cleanup function that restores the original value.
// The target field is specified via either the legacy "field" parameter (which targets spec.{field})
// or the "path" parameter (a dot-notation path like "spec.replicas" or "metadata.labels.app").
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

	// Resolve the target path segments.
	// Legacy "field" param targets spec.{field}. New "path" param is a full dot-notation path.
	pathSegments := resolveMutationPath(spec.Parameters)

	// Read the original value at the target path
	originalValue, _, _ := unstructured.NestedFieldCopy(obj.Object, pathSegments...)

	// Build rollback data for crash-safe recovery
	rollbackInfo := map[string]any{
		"apiVersion":    spec.Parameters["apiVersion"],
		"kind":          spec.Parameters["kind"],
		"path":          strings.Join(pathSegments, "."),
		"originalValue": originalValue,
	}
	// Keep legacy "field" key for backward compatibility with existing rollback data
	if spec.Parameters["field"] != "" {
		rollbackInfo["field"] = spec.Parameters["field"]
	}
	rollbackStr, err := safety.WrapRollbackData(rollbackInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing rollback data for %s/%s: %w", spec.Parameters["kind"], spec.Parameters["name"], err)
	}

	// Build annotations map with rollback data
	annotationsMap := map[string]any{
		safety.RollbackAnnotationKey: rollbackStr,
	}

	// Build labels map with chaos labels
	chaosLabels := safety.ChaosLabels(string(v1alpha1.CRDMutation))
	labelsMap := make(map[string]any, len(chaosLabels))
	for k, v := range chaosLabels {
		labelsMap[k] = v
	}

	// Parse the value with JSON-aware type detection so that numeric and
	// boolean values are sent with their correct JSON types instead of
	// always being injected as strings.
	typedValue := parseTypedValue(spec.Parameters["value"])

	// Build nested map structure for the merge patch from path segments
	valueMap := buildNestedMap(pathSegments, typedValue)

	// Apply mutation via merge patch including rollback annotation and chaos labels
	patchMap := deepMerge(valueMap, map[string]any{
		"metadata": map[string]any{
			"annotations": annotationsMap,
			"labels":      labelsMap,
		},
	})
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

	pathStr := strings.Join(pathSegments, ".")
	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.CRDMutation, key.String(), "mutated",
			map[string]string{
				"path":  pathStr,
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
		restoreAnnotations := map[string]any{
			safety.RollbackAnnotationKey: nil,
		}
		restoreLabels := make(map[string]any)
		for k := range chaosLabels {
			restoreLabels[k] = nil
		}

		// When originalValue is nil, JSON merge patch serializes it as null,
		// which removes the key.
		restoreValueMap := buildNestedMap(pathSegments, originalValue)
		restorePatchMap := deepMerge(restoreValueMap, map[string]any{
			"metadata": map[string]any{
				"annotations": restoreAnnotations,
				"labels":      restoreLabels,
			},
		})
		restorePatch, err := json.Marshal(restorePatchMap)
		if err != nil {
			return fmt.Errorf("building restore patch: %w", err)
		}

		return m.client.Patch(ctx, current, client.RawPatch(types.MergePatchType, restorePatch))
	}

	return cleanup, events, nil
}

// Revert restores the original field value on the custom resource using a merge patch.
// It is idempotent: if no rollback annotation is present, it returns nil.
func (m *CRDMutationInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(spec.Parameters["apiVersion"])
	obj.SetKind(spec.Parameters["kind"])

	key := types.NamespacedName{
		Name:      spec.Parameters["name"],
		Namespace: namespace,
	}

	if err := m.client.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting resource %s/%s for revert: %w", spec.Parameters["kind"], spec.Parameters["name"], err)
	}

	// Check for rollback annotation — if absent, already reverted
	annotations := obj.GetAnnotations()
	rollbackStr, ok := annotations[safety.RollbackAnnotationKey]
	if !ok {
		return nil
	}

	var rollbackInfo map[string]any
	if err := safety.UnwrapRollbackData(rollbackStr, &rollbackInfo); err != nil {
		return fmt.Errorf("unwrapping rollback data for %s/%s: %w", spec.Parameters["kind"], spec.Parameters["name"], err)
	}

	// Resolve path from rollback data. New format stores "path", legacy stores "field" (implies spec.{field}).
	var pathSegments []string
	if pathStr, ok := rollbackInfo["path"].(string); ok && pathStr != "" {
		pathSegments = strings.Split(pathStr, ".")
	} else if fieldName, ok := rollbackInfo["field"].(string); ok && fieldName != "" {
		pathSegments = []string{"spec", fieldName}
	} else {
		return fmt.Errorf("rollback data missing 'path' or 'field' key for %s/%s", spec.Parameters["kind"], spec.Parameters["name"])
	}
	originalValue := rollbackInfo["originalValue"]

	// Build chaos labels to remove
	chaosLabels := safety.ChaosLabels(string(v1alpha1.CRDMutation))
	restoreAnnotations := map[string]any{
		safety.RollbackAnnotationKey: nil,
	}
	restoreLabels := make(map[string]any)
	for k := range chaosLabels {
		restoreLabels[k] = nil
	}

	restoreValueMap := buildNestedMap(pathSegments, originalValue)
	restorePatchMap := deepMerge(restoreValueMap, map[string]any{
		"metadata": map[string]any{
			"annotations": restoreAnnotations,
			"labels":      restoreLabels,
		},
	})
	restorePatch, err := json.Marshal(restorePatchMap)
	if err != nil {
		return fmt.Errorf("building restore patch: %w", err)
	}

	return m.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, restorePatch))
}

// resolveMutationPath returns the path segments for the mutation target.
// If "path" parameter is set, it splits on dots. Otherwise falls back to
// legacy "field" parameter which implies ["spec", fieldName].
func resolveMutationPath(params map[string]string) []string {
	if p := params["path"]; p != "" {
		return strings.Split(p, ".")
	}
	return []string{"spec", params["field"]}
}

// buildNestedMap creates a nested map structure from path segments and a leaf value.
// e.g. ["spec", "template", "replicas"] with value 3 produces:
// {"spec": {"template": {"replicas": 3}}}
func buildNestedMap(segments []string, value any) map[string]any {
	if len(segments) == 0 {
		return nil
	}
	if len(segments) == 1 {
		return map[string]any{segments[0]: value}
	}
	return map[string]any{segments[0]: buildNestedMap(segments[1:], value)}
}

// deepMerge recursively merges two maps. When both maps have the same key
// and both values are maps, they are merged recursively. Otherwise the value
// from b takes precedence.
func deepMerge(a, b map[string]any) map[string]any {
	result := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		if existing, ok := result[k]; ok {
			existingMap, existingIsMap := existing.(map[string]any)
			newMap, newIsMap := v.(map[string]any)
			if existingIsMap && newIsMap {
				result[k] = deepMerge(existingMap, newMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// parseTypedValue attempts to interpret a string as a JSON literal. If the
// string is valid JSON (number, boolean, null, array, or object) the decoded
// Go value is returned. Otherwise the original string is returned as-is.
// This ensures that parameter values like "999" become the integer 999 and
// "true" becomes the boolean true in the resulting merge patch, matching the
// types expected by Kubernetes API validation.
func parseTypedValue(raw string) any {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// Not valid JSON – treat as a plain string.
		return raw
	}
	// json.Unmarshal on a JSON string (e.g. `"hello"`) returns a Go string.
	// We only want to use the parsed value when it is a *different* type,
	// because bare words that happen to be valid JSON strings (quoted) are
	// unlikely in ChaosExperiment parameters. For an unquoted value like
	// `hello` the Unmarshal above already fails, so we just return parsed
	// unconditionally here.
	return parsed
}
