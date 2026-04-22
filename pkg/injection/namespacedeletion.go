package injection

import (
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NamespaceDeletionInjector struct {
	client client.Client
}

func NewNamespaceDeletionInjector(c client.Client) *NamespaceDeletionInjector {
	return &NamespaceDeletionInjector{client: c}
}

func (n *NamespaceDeletionInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateNamespaceDeletionParams(spec)
}

func rollbackConfigMapName(namespace string) string {
	return "chaos-rollback-ns-" + namespace
}

func (n *NamespaceDeletionInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	targetNs := spec.Parameters["namespace"]
	safeNamespace := namespace

	// Prevent deleting the namespace where rollback data is stored
	if targetNs == safeNamespace {
		return nil, nil, fmt.Errorf("NamespaceDeletion cannot target namespace %q because it is the same namespace used for rollback data storage", targetNs)
	}

	// Get the namespace object
	var ns corev1.Namespace
	if err := n.client.Get(ctx, types.NamespacedName{Name: targetNs}, &ns); err != nil {
		return nil, nil, fmt.Errorf("getting namespace %q: %w", targetNs, err)
	}

	// Snapshot labels and annotations
	labelsJSON, err := json.Marshal(ns.Labels)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing namespace labels: %w", err)
	}
	annotationsJSON, err := json.Marshal(ns.Annotations)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing namespace annotations: %w", err)
	}

	// Count resources in the namespace for the event
	var deployList appsv1.DeploymentList
	var svcList corev1.ServiceList
	var cmList corev1.ConfigMapList
	var podList corev1.PodList

	listOpts := []client.ListOption{client.InNamespace(targetNs)}
	deployCount, svcCount, cmCount, podCount := 0, 0, 0, 0

	if err := n.client.List(ctx, &deployList, listOpts...); err == nil {
		deployCount = len(deployList.Items)
	}
	if err := n.client.List(ctx, &svcList, listOpts...); err == nil {
		svcCount = len(svcList.Items)
	}
	if err := n.client.List(ctx, &cmList, listOpts...); err == nil {
		cmCount = len(cmList.Items)
	}
	if err := n.client.List(ctx, &podList, listOpts...); err == nil {
		podCount = len(podList.Items)
	}

	// Store rollback data in ConfigMap in safe namespace.
	// If a stale rollback ConfigMap exists from a prior crashed experiment,
	// update it in-place (using its resourceVersion for optimistic concurrency)
	// to avoid a TOCTOU race between Get and Delete+Create.
	rollbackCMName := rollbackConfigMapName(targetNs)
	chaosLabels := safety.ChaosLabels(string(v1alpha1.NamespaceDeletion))
	rollbackData := map[string]string{
		"labels":      string(labelsJSON),
		"annotations": string(annotationsJSON),
	}

	var existingCM corev1.ConfigMap
	if err := n.client.Get(ctx, types.NamespacedName{Name: rollbackCMName, Namespace: safeNamespace}, &existingCM); err == nil {
		if existingCM.Labels[safety.ManagedByLabel] != chaosLabels[safety.ManagedByLabel] {
			return nil, nil, fmt.Errorf("ConfigMap %q already exists in namespace %q and is not chaos-managed; refusing to overwrite", rollbackCMName, safeNamespace)
		}
		// Update in-place: resourceVersion from the Get ensures no concurrent modification
		existingCM.Labels = chaosLabels
		existingCM.Data = rollbackData
		if err := n.client.Update(ctx, &existingCM); err != nil {
			return nil, nil, fmt.Errorf("updating stale rollback ConfigMap: %w", err)
		}
	} else if apierrors.IsNotFound(err) {
		rollbackCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rollbackCMName,
				Namespace: safeNamespace,
				Labels:    chaosLabels,
			},
			Data: rollbackData,
		}
		if err := n.client.Create(ctx, rollbackCM); err != nil {
			return nil, nil, fmt.Errorf("creating rollback ConfigMap: %w", err)
		}
	} else {
		return nil, nil, fmt.Errorf("checking for existing rollback ConfigMap: %w", err)
	}

	// Delete the namespace
	if err := n.client.Delete(ctx, &ns); err != nil {
		// Best-effort cleanup of the rollback ConfigMap we just created/updated
		cleanupCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rollbackCMName,
				Namespace: safeNamespace,
			},
		}
		_ = n.client.Delete(ctx, cleanupCM)
		return nil, nil, fmt.Errorf("deleting namespace %q: %w", targetNs, err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.NamespaceDeletion, targetNs, "deleted",
			map[string]string{
				"namespace":   targetNs,
				"deployments": fmt.Sprintf("%d", deployCount),
				"services":    fmt.Sprintf("%d", svcCount),
				"configmaps":  fmt.Sprintf("%d", cmCount),
				"pods":        fmt.Sprintf("%d", podCount),
			}),
	}

	cleanup := func(ctx context.Context) error {
		return n.restoreNamespace(ctx, targetNs, safeNamespace)
	}

	return cleanup, events, nil
}

func (n *NamespaceDeletionInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	targetNs := spec.Parameters["namespace"]
	return n.restoreNamespace(ctx, targetNs, namespace)
}

func (n *NamespaceDeletionInjector) restoreNamespace(ctx context.Context, targetNs, safeNamespace string) error {
	cmName := rollbackConfigMapName(targetNs)
	cmKey := types.NamespacedName{Name: cmName, Namespace: safeNamespace}

	var rollbackCM corev1.ConfigMap
	if err := n.client.Get(ctx, cmKey, &rollbackCM); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting rollback ConfigMap: %w", err)
	}

	// If namespace already exists, check its phase before assuming recovery
	var existingNs corev1.Namespace
	if err := n.client.Get(ctx, types.NamespacedName{Name: targetNs}, &existingNs); err == nil {
		if existingNs.Status.Phase == corev1.NamespaceTerminating {
			return fmt.Errorf("namespace %q is still in Terminating phase; wait for deletion to complete before reverting", targetNs)
		}
		// Namespace is Active (operator recreated it), just clean up ConfigMap
		return n.client.Delete(ctx, &rollbackCM)
	}

	// Recreate namespace with stored metadata
	var storedLabels map[string]string
	var storedAnnotations map[string]string

	if labelsData := rollbackCM.Data["labels"]; labelsData != "" {
		if err := json.Unmarshal([]byte(labelsData), &storedLabels); err != nil {
			return fmt.Errorf("deserializing namespace labels: %w", err)
		}
	}
	if annotationsData := rollbackCM.Data["annotations"]; annotationsData != "" {
		if err := json.Unmarshal([]byte(annotationsData), &storedAnnotations); err != nil {
			return fmt.Errorf("deserializing namespace annotations: %w", err)
		}
	}

	newNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        targetNs,
			Labels:      storedLabels,
			Annotations: storedAnnotations,
		},
	}
	if err := n.client.Create(ctx, newNs); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Race: namespace was recreated between our Get and Create (e.g. by the operator).
			// This is the desired outcome, so proceed to clean up the rollback ConfigMap.
		} else {
			return fmt.Errorf("recreating namespace %q: %w", targetNs, err)
		}
	}

	return n.client.Delete(ctx, &rollbackCM)
}
