package injection

import (
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LabelStompingInjector modifies or removes labels on operator-managed resources
// to test whether the operator's label-based reconciliation detects and corrects drift.
type LabelStompingInjector struct {
	client client.Client
}

func NewLabelStompingInjector(c client.Client) *LabelStompingInjector {
	return &LabelStompingInjector{client: c}
}

func (l *LabelStompingInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateLabelStompingParams(spec)
}

func (l *LabelStompingInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(spec.Parameters["apiVersion"])
	obj.SetKind(spec.Parameters["kind"])

	key := types.NamespacedName{
		Name:      spec.Parameters["name"],
		Namespace: namespace,
	}

	if err := l.client.Get(ctx, key, obj); err != nil {
		return nil, nil, fmt.Errorf("getting resource %s/%s: %w", spec.Parameters["kind"], spec.Parameters["name"], err)
	}

	labelKey := spec.Parameters["labelKey"]
	action := spec.Parameters["action"]
	newValue := spec.Parameters["newValue"]
	if newValue == "" {
		newValue = "chaos-stomped"
	}

	// Read current label value
	currentLabels := obj.GetLabels()
	originalValue := ""
	existed := false
	if currentLabels != nil {
		if v, ok := currentLabels[labelKey]; ok {
			originalValue = v
			existed = true
		}
	}

	// Deleting a label that doesn't exist is a no-op, reject it
	if action == "delete" && !existed {
		return nil, nil, fmt.Errorf("label %q does not exist on %s/%s; nothing to delete", labelKey, spec.Parameters["kind"], spec.Parameters["name"])
	}

	// Store rollback data
	rollbackInfo := map[string]any{
		"labelKey":      labelKey,
		"originalValue": originalValue,
		"existed":       existed,
	}
	rollbackStr, err := safety.WrapRollbackData(rollbackInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("wrapping rollback data: %w", err)
	}

	// Build patch
	chaosLabels := safety.ChaosLabels(string(v1alpha1.LabelStomping))
	labelsMap := make(map[string]any, len(chaosLabels)+1)
	for k, v := range chaosLabels {
		labelsMap[k] = v
	}

	if action == "overwrite" {
		labelsMap[labelKey] = newValue
	} else {
		labelsMap[labelKey] = nil // JSON merge patch: null removes the key
	}

	patchMap := map[string]any{
		"metadata": map[string]any{
			"labels": labelsMap,
			"annotations": map[string]any{
				safety.RollbackAnnotationKey: rollbackStr,
			},
		},
	}
	patch, err := json.Marshal(patchMap)
	if err != nil {
		return nil, nil, fmt.Errorf("building label stomp patch: %w", err)
	}
	if err := l.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return nil, nil, fmt.Errorf("patching label %q: %w", labelKey, err)
	}

	eventNewValue := newValue
	if action == "delete" {
		eventNewValue = "<deleted>"
	}
	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.LabelStomping, key.String(), "label-stomped",
			map[string]string{
				"labelKey":      labelKey,
				"action":        action,
				"originalValue": originalValue,
				"newValue":      eventNewValue,
			}),
	}

	apiVersion := spec.Parameters["apiVersion"]
	kind := spec.Parameters["kind"]

	cleanup := func(ctx context.Context) error {
		current := &unstructured.Unstructured{}
		current.SetAPIVersion(apiVersion)
		current.SetKind(kind)
		if err := l.client.Get(ctx, key, current); err != nil {
			return fmt.Errorf("re-fetching resource for cleanup: %w", err)
		}

		restoreLabels := make(map[string]any)
		for k := range chaosLabels {
			restoreLabels[k] = nil
		}
		if existed {
			restoreLabels[labelKey] = originalValue
		} else {
			restoreLabels[labelKey] = nil
		}

		restorePatchMap := map[string]any{
			"metadata": map[string]any{
				"labels": restoreLabels,
				"annotations": map[string]any{
					safety.RollbackAnnotationKey: nil,
				},
			},
		}
		restorePatch, err := json.Marshal(restorePatchMap)
		if err != nil {
			return fmt.Errorf("building restore patch: %w", err)
		}
		return l.client.Patch(ctx, current, client.RawPatch(types.MergePatchType, restorePatch))
	}

	return cleanup, events, nil
}

func (l *LabelStompingInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(spec.Parameters["apiVersion"])
	obj.SetKind(spec.Parameters["kind"])

	key := types.NamespacedName{
		Name:      spec.Parameters["name"],
		Namespace: namespace,
	}

	if err := l.client.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting resource for revert: %w", err)
	}

	annotations := obj.GetAnnotations()
	rollbackStr, ok := annotations[safety.RollbackAnnotationKey]
	if !ok {
		return nil
	}

	var rollbackInfo map[string]any
	if err := safety.UnwrapRollbackData(rollbackStr, &rollbackInfo); err != nil {
		return fmt.Errorf("unwrapping rollback data: %w", err)
	}

	labelKey, _ := rollbackInfo["labelKey"].(string)
	originalValue, _ := rollbackInfo["originalValue"].(string)
	existed, _ := rollbackInfo["existed"].(bool)

	chaosLabels := safety.ChaosLabels(string(v1alpha1.LabelStomping))
	restoreLabels := make(map[string]any)
	for k := range chaosLabels {
		restoreLabels[k] = nil
	}
	if existed {
		restoreLabels[labelKey] = originalValue
	} else {
		restoreLabels[labelKey] = nil
	}

	restorePatchMap := map[string]any{
		"metadata": map[string]any{
			"labels": restoreLabels,
			"annotations": map[string]any{
				safety.RollbackAnnotationKey: nil,
			},
		},
	}
	restorePatch, err := json.Marshal(restorePatchMap)
	if err != nil {
		return fmt.Errorf("building restore patch: %w", err)
	}
	return l.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, restorePatch))
}
