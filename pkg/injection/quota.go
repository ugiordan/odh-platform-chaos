package injection

import (
	"context"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// QuotaExhaustionInjector creates a restrictive ResourceQuota on the target
// namespace so the operator cannot create or update resources.
type QuotaExhaustionInjector struct {
	client client.Client
}

// NewQuotaExhaustionInjector creates a new QuotaExhaustionInjector.
func NewQuotaExhaustionInjector(c client.Client) *QuotaExhaustionInjector {
	return &QuotaExhaustionInjector{client: c}
}

func (q *QuotaExhaustionInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateQuotaExhaustionParams(spec)
}

// Inject creates a ResourceQuota with tight limits on the target namespace.
func (q *QuotaExhaustionInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	quotaName := spec.Parameters["quotaName"]

	// Check if a quota with this name already exists
	existing := &corev1.ResourceQuota{}
	err := q.client.Get(ctx, types.NamespacedName{Name: quotaName, Namespace: namespace}, existing)
	if err == nil {
		return nil, nil, fmt.Errorf("ResourceQuota %q already exists in namespace %q; refusing to overwrite", quotaName, namespace)
	}
	if !apierrors.IsNotFound(err) {
		return nil, nil, fmt.Errorf("checking for existing ResourceQuota: %w", err)
	}

	// Build resource limits from parameters
	hard := corev1.ResourceList{}
	limitMap := map[string]corev1.ResourceName{
		"cpu":        corev1.ResourceCPU,
		"memory":     corev1.ResourceMemory,
		"pods":       corev1.ResourcePods,
		"services":   corev1.ResourceServices,
		"configmaps": corev1.ResourceConfigMaps,
		"secrets":    corev1.ResourceSecrets,
	}

	for param, resourceName := range limitMap {
		if val := spec.Parameters[param]; val != "" {
			qty, err := resource.ParseQuantity(val)
			if err != nil {
				return nil, nil, fmt.Errorf("parsing %s limit %q: %w", param, val, err)
			}
			hard[resourceName] = qty
		}
	}

	chaosLabels := safety.ChaosLabels(string(v1alpha1.QuotaExhaustion))

	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      quotaName,
			Namespace: namespace,
			Labels:    chaosLabels,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	if err := q.client.Create(ctx, quota); err != nil {
		return nil, nil, fmt.Errorf("creating ResourceQuota %q: %w", quotaName, err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.QuotaExhaustion, namespace+"/"+quotaName, "created",
			map[string]string{
				"quotaName": quotaName,
				"namespace": namespace,
			}),
	}

	cleanup := func(ctx context.Context) error {
		toDelete := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      quotaName,
				Namespace: namespace,
			},
		}
		if err := q.client.Delete(ctx, toDelete); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("deleting ResourceQuota %q: %w", quotaName, err)
		}
		return nil
	}

	return cleanup, events, nil
}

// Revert deletes the injected ResourceQuota. Idempotent.
func (q *QuotaExhaustionInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	quotaName := spec.Parameters["quotaName"]

	existing := &corev1.ResourceQuota{}
	err := q.client.Get(ctx, types.NamespacedName{Name: quotaName, Namespace: namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting ResourceQuota for revert: %w", err)
	}

	// Only delete if it has our chaos labels
	labels := existing.GetLabels()
	if labels[safety.ManagedByLabel] != safety.ManagedByValue {
		return fmt.Errorf("ResourceQuota %q is not managed by chaos framework; refusing to delete", quotaName)
	}

	return q.client.Delete(ctx, existing)
}
