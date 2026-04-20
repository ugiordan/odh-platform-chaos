package injection

import (
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OwnerRefOrphanInjector removes ownerReferences from operator-managed resources
// to verify the operator re-adopts them within the recovery timeout.
type OwnerRefOrphanInjector struct {
	client client.Client
}

// NewOwnerRefOrphanInjector creates a new OwnerRefOrphanInjector.
func NewOwnerRefOrphanInjector(c client.Client) *OwnerRefOrphanInjector {
	return &OwnerRefOrphanInjector{client: c}
}

func (o *OwnerRefOrphanInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateOwnerRefOrphanParams(spec)
}

// Inject removes all ownerReferences from the target resource and stores them
// in a rollback annotation for crash-safe recovery.
func (o *OwnerRefOrphanInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(spec.Parameters["apiVersion"])
	obj.SetKind(spec.Parameters["kind"])

	key := types.NamespacedName{
		Name:      spec.Parameters["name"],
		Namespace: namespace,
	}

	if err := o.client.Get(ctx, key, obj); err != nil {
		return nil, nil, fmt.Errorf("getting resource %s/%s: %w", spec.Parameters["kind"], spec.Parameters["name"], err)
	}

	// Save original ownerReferences for rollback
	originalOwnerRefs := obj.GetOwnerReferences()
	if len(originalOwnerRefs) == 0 {
		return nil, nil, fmt.Errorf("resource %s/%s has no ownerReferences to orphan", spec.Parameters["kind"], spec.Parameters["name"])
	}

	// Serialize ownerReferences for rollback
	ownerRefData, err := json.Marshal(originalOwnerRefs)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing ownerReferences: %w", err)
	}

	rollbackInfo := map[string]any{
		"apiVersion":      spec.Parameters["apiVersion"],
		"kind":            spec.Parameters["kind"],
		"ownerReferences": string(ownerRefData),
	}
	rollbackStr, err := safety.WrapRollbackData(rollbackInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("wrapping rollback data: %w", err)
	}

	// Build patch: remove ownerReferences, add rollback annotation + chaos labels
	chaosLabels := safety.ChaosLabels(string(v1alpha1.OwnerRefOrphan))
	labelsMap := make(map[string]any, len(chaosLabels))
	for k, v := range chaosLabels {
		labelsMap[k] = v
	}

	// With JSON merge patch, setting ownerReferences to an empty array clears them.
	patchMap := map[string]any{
		"metadata": map[string]any{
			"ownerReferences": []any{},
			"annotations": map[string]any{
				safety.RollbackAnnotationKey: rollbackStr,
			},
			"labels": labelsMap,
		},
	}
	patch, err := json.Marshal(patchMap)
	if err != nil {
		return nil, nil, fmt.Errorf("building orphan patch: %w", err)
	}
	if err := o.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return nil, nil, fmt.Errorf("removing ownerReferences: %w", err)
	}

	apiVersion := spec.Parameters["apiVersion"]
	kind := spec.Parameters["kind"]

	ownerNames := make([]string, len(originalOwnerRefs))
	for i, ref := range originalOwnerRefs {
		ownerNames[i] = ref.Kind + "/" + ref.Name
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.OwnerRefOrphan, key.String(), "orphaned",
			map[string]string{
				"removedOwners": fmt.Sprintf("%v", ownerNames),
				"count":         fmt.Sprintf("%d", len(originalOwnerRefs)),
			}),
	}

	// Cleanup restores original ownerReferences
	cleanup := func(ctx context.Context) error {
		current := &unstructured.Unstructured{}
		current.SetAPIVersion(apiVersion)
		current.SetKind(kind)
		if err := o.client.Get(ctx, key, current); err != nil {
			return fmt.Errorf("re-fetching resource for cleanup: %w", err)
		}

		// If the operator has already re-adopted (ownerReferences are non-empty),
		// just remove the chaos metadata
		currentOwnerRefs := current.GetOwnerReferences()

		restoreAnnotations := map[string]any{
			safety.RollbackAnnotationKey: nil,
		}
		restoreLabels := make(map[string]any)
		for k := range chaosLabels {
			restoreLabels[k] = nil
		}

		restorePatchMap := map[string]any{
			"metadata": map[string]any{
				"annotations": restoreAnnotations,
				"labels":      restoreLabels,
			},
		}

		// Only restore ownerReferences if operator hasn't already re-adopted
		if len(currentOwnerRefs) == 0 {
			// Convert to unstructured format for merge patch
			ownerRefsUnstructured := make([]any, len(originalOwnerRefs))
			for i, ref := range originalOwnerRefs {
				refMap := map[string]any{
					"apiVersion": ref.APIVersion,
					"kind":       ref.Kind,
					"name":       ref.Name,
					"uid":        string(ref.UID),
				}
				if ref.Controller != nil {
					refMap["controller"] = *ref.Controller
				}
				if ref.BlockOwnerDeletion != nil {
					refMap["blockOwnerDeletion"] = *ref.BlockOwnerDeletion
				}
				ownerRefsUnstructured[i] = refMap
			}
			restorePatchMap["metadata"].(map[string]any)["ownerReferences"] = ownerRefsUnstructured
		}

		restorePatch, err := json.Marshal(restorePatchMap)
		if err != nil {
			return fmt.Errorf("building restore patch: %w", err)
		}
		return o.client.Patch(ctx, current, client.RawPatch(types.MergePatchType, restorePatch))
	}

	return cleanup, events, nil
}

// Revert restores ownerReferences from the rollback annotation.
func (o *OwnerRefOrphanInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(spec.Parameters["apiVersion"])
	obj.SetKind(spec.Parameters["kind"])

	key := types.NamespacedName{
		Name:      spec.Parameters["name"],
		Namespace: namespace,
	}

	if err := o.client.Get(ctx, key, obj); err != nil {
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

	ownerRefJSON, ok := rollbackInfo["ownerReferences"].(string)
	if !ok {
		return fmt.Errorf("rollback data missing ownerReferences")
	}

	var ownerRefs []metav1.OwnerReference
	if err := json.Unmarshal([]byte(ownerRefJSON), &ownerRefs); err != nil {
		return fmt.Errorf("deserializing ownerReferences: %w", err)
	}

	// Build restore patch
	chaosLabels := safety.ChaosLabels(string(v1alpha1.OwnerRefOrphan))
	restoreAnnotations := map[string]any{
		safety.RollbackAnnotationKey: nil,
	}
	restoreLabels := make(map[string]any)
	for k := range chaosLabels {
		restoreLabels[k] = nil
	}

	restorePatchMap := map[string]any{
		"metadata": map[string]any{
			"annotations": restoreAnnotations,
			"labels":      restoreLabels,
		},
	}

	// Only restore ownerReferences if operator hasn't already re-adopted
	currentOwnerRefs := obj.GetOwnerReferences()
	if len(currentOwnerRefs) == 0 {
		ownerRefsUnstructured := make([]any, len(ownerRefs))
		for i, ref := range ownerRefs {
			refMap := map[string]any{
				"apiVersion": ref.APIVersion,
				"kind":       ref.Kind,
				"name":       ref.Name,
				"uid":        string(ref.UID),
			}
			if ref.Controller != nil {
				refMap["controller"] = *ref.Controller
			}
			if ref.BlockOwnerDeletion != nil {
				refMap["blockOwnerDeletion"] = *ref.BlockOwnerDeletion
			}
			ownerRefsUnstructured[i] = refMap
		}
		restorePatchMap["metadata"].(map[string]any)["ownerReferences"] = ownerRefsUnstructured
	}

	restorePatch, err := json.Marshal(restorePatchMap)
	if err != nil {
		return fmt.Errorf("building restore patch: %w", err)
	}

	return o.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, restorePatch))
}
