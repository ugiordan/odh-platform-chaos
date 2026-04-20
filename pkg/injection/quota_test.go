package injection

import (
	"context"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestQuotaExhaustionInjectAndCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create a namespace so the quota has somewhere to live
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ns).Build()
	injector := NewQuotaExhaustionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.QuotaExhaustion,
		Parameters: map[string]string{
			"quotaName": "chaos-quota",
			"cpu":       "10m",
			"memory":    "1Mi",
			"pods":      "0",
		},
	}

	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	require.Len(t, events, 1)
	assert.Equal(t, "created", events[0].Action)

	// Verify ResourceQuota was created
	quota := &corev1.ResourceQuota{}
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-quota", Namespace: "test-ns"}, quota))
	assert.Equal(t, resource.MustParse("10m"), quota.Spec.Hard[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("1Mi"), quota.Spec.Hard[corev1.ResourceMemory])
	assert.Equal(t, resource.MustParse("0"), quota.Spec.Hard[corev1.ResourcePods])

	// Verify chaos labels
	assert.Equal(t, safety.ManagedByValue, quota.Labels[safety.ManagedByLabel])

	// Cleanup deletes the quota
	require.NoError(t, cleanup(ctx))

	err = fakeClient.Get(ctx, types.NamespacedName{Name: "chaos-quota", Namespace: "test-ns"}, quota)
	assert.Error(t, err) // should be NotFound
}

func TestQuotaExhaustionRejectsDuplicate(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	existingQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-quota",
			Namespace: "test-ns",
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100"),
			},
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingQuota).Build()
	injector := NewQuotaExhaustionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.QuotaExhaustion,
		Parameters: map[string]string{
			"quotaName": "existing-quota",
			"pods":      "0",
		},
	}

	_, _, err := injector.Inject(ctx, spec, "test-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestQuotaExhaustionRevert(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	chaosLabels := safety.ChaosLabels(string(v1alpha1.QuotaExhaustion))
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chaos-quota",
			Namespace: "test-ns",
			Labels:    chaosLabels,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourcePods: resource.MustParse("0"),
			},
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(quota).Build()
	injector := NewQuotaExhaustionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.QuotaExhaustion,
		Parameters: map[string]string{
			"quotaName": "chaos-quota",
		},
	}

	err := injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Idempotent
	err = injector.Revert(ctx, spec, "test-ns")
	assert.NoError(t, err)
}

func TestQuotaExhaustionRevertRefusesNonChaos(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user-quota",
			Namespace: "test-ns",
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourcePods: resource.MustParse("10"),
			},
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(quota).Build()
	injector := NewQuotaExhaustionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.QuotaExhaustion,
		Parameters: map[string]string{
			"quotaName": "user-quota",
		},
	}

	err := injector.Revert(ctx, spec, "test-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not managed by chaos")
}

func TestQuotaExhaustionValidate(t *testing.T) {
	injector := &QuotaExhaustionInjector{}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	tests := []struct {
		name    string
		spec    v1alpha1.InjectionSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid with pods limit",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.QuotaExhaustion,
				Parameters: map[string]string{
					"quotaName": "chaos-quota",
					"pods":      "0",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with multiple limits",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.QuotaExhaustion,
				Parameters: map[string]string{
					"quotaName": "chaos-quota",
					"cpu":       "10m",
					"memory":    "1Mi",
				},
			},
			wantErr: false,
		},
		{
			name: "missing quotaName",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.QuotaExhaustion,
				Parameters: map[string]string{
					"pods": "0",
				},
			},
			wantErr: true,
			errMsg:  "quotaName",
		},
		{
			name: "no resource limits",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.QuotaExhaustion,
				Parameters: map[string]string{
					"quotaName": "chaos-quota",
				},
			},
			wantErr: true,
			errMsg:  "at least one resource limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := injector.Validate(tt.spec, blast)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
