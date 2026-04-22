package injection

import (
	"context"
	"encoding/json"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNamespaceDeletionInject(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	victimNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "victim-ns",
			Labels: map[string]string{
				"env": "test",
			},
			Annotations: map[string]string{
				"team": "chaos",
			},
		},
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: "victim-ns",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "test:latest"},
					},
				},
			},
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "victim-ns",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80},
			},
		},
	}

	safeNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "safe-ns",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(victimNs, safeNs, deploy, svc).
		Build()

	injector := NewNamespaceDeletionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.NamespaceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"namespace": "victim-ns",
		},
	}

	cleanup, events, err := injector.Inject(ctx, spec, "safe-ns")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	require.Len(t, events, 1)

	// Verify namespace was deleted
	var ns corev1.Namespace
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "victim-ns"}, &ns)
	assert.True(t, apierrors.IsNotFound(err), "namespace should be deleted")

	// Verify rollback ConfigMap was created in safe namespace
	var rollbackCM corev1.ConfigMap
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      rollbackConfigMapName("victim-ns"),
		Namespace: "safe-ns",
	}, &rollbackCM)
	require.NoError(t, err)

	// Verify chaos labels
	assert.Equal(t, safety.ManagedByValue, rollbackCM.Labels[safety.ManagedByLabel])
	assert.Equal(t, string(v1alpha1.NamespaceDeletion), rollbackCM.Labels[safety.ChaosTypeLabel])

	// Verify stored metadata
	var storedLabels map[string]string
	var storedAnnotations map[string]string
	require.NoError(t, json.Unmarshal([]byte(rollbackCM.Data["labels"]), &storedLabels))
	require.NoError(t, json.Unmarshal([]byte(rollbackCM.Data["annotations"]), &storedAnnotations))
	assert.Equal(t, "test", storedLabels["env"])
	assert.Equal(t, "chaos", storedAnnotations["team"])

	// Verify event has resource counts
	assert.Equal(t, "victim-ns", events[0].Details["namespace"])
	assert.Equal(t, "1", events[0].Details["deployments"])
	assert.Equal(t, "1", events[0].Details["services"])

	// Run cleanup
	require.NoError(t, cleanup(ctx))

	// Verify namespace was recreated with original labels and annotations
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "victim-ns"}, &ns)
	require.NoError(t, err)
	assert.Equal(t, "test", ns.Labels["env"])
	assert.Equal(t, "chaos", ns.Annotations["team"])

	// Verify rollback ConfigMap was deleted
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      rollbackConfigMapName("victim-ns"),
		Namespace: "safe-ns",
	}, &rollbackCM)
	assert.True(t, apierrors.IsNotFound(err), "rollback ConfigMap should be deleted")
}

func TestNamespaceDeletionInjectNamespaceNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	safeNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "safe-ns",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(safeNs).
		Build()

	injector := NewNamespaceDeletionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.NamespaceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"namespace": "missing-ns",
		},
	}

	cleanup, events, err := injector.Inject(ctx, spec, "safe-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting namespace")
	assert.Nil(t, cleanup)
	assert.Nil(t, events)
}

func TestNamespaceDeletionInjectSelfNamespaceGuard(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	// Both target and safe namespace are the same
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "same-ns"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns).
		Build()

	injector := NewNamespaceDeletionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.NamespaceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"namespace": "same-ns",
		},
	}

	cleanup, events, err := injector.Inject(ctx, spec, "same-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "same namespace used for rollback")
	assert.Nil(t, cleanup)
	assert.Nil(t, events)
}

func TestNamespaceDeletionInjectStaleConfigMapCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	victimNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "victim-ns"},
	}
	safeNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "safe-ns"},
	}

	// Stale chaos-managed rollback ConfigMap from a prior crashed experiment
	staleCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rollbackConfigMapName("victim-ns"),
			Namespace: "safe-ns",
			Labels:    safety.ChaosLabels(string(v1alpha1.NamespaceDeletion)),
		},
		Data: map[string]string{
			"labels":      "{}",
			"annotations": "{}",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(victimNs, safeNs, staleCM).
		Build()

	injector := NewNamespaceDeletionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.NamespaceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"namespace": "victim-ns",
		},
	}

	// Inject should succeed despite the stale ConfigMap
	cleanup, events, err := injector.Inject(ctx, spec, "safe-ns")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	require.Len(t, events, 1)
}

func TestNamespaceDeletionInjectStaleConfigMapNonChaosManaged(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	victimNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "victim-ns"},
	}
	safeNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "safe-ns"},
	}

	// Non-chaos-managed ConfigMap with the same name (should NOT be deleted)
	nonChaosCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rollbackConfigMapName("victim-ns"),
			Namespace: "safe-ns",
			Labels:    map[string]string{"team": "platform"},
		},
		Data: map[string]string{"important": "data"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(victimNs, safeNs, nonChaosCM).
		Build()

	injector := NewNamespaceDeletionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.NamespaceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"namespace": "victim-ns",
		},
	}

	// Should refuse to overwrite the non-chaos-managed ConfigMap
	cleanup, events, err := injector.Inject(ctx, spec, "safe-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not chaos-managed")
	assert.Nil(t, cleanup)
	assert.Nil(t, events)
}

func TestNamespaceDeletionRevert(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	safeNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "safe-ns",
		},
	}

	// Simulate rollback ConfigMap left from a crash
	storedLabels := map[string]string{"env": "production"}
	storedAnnotations := map[string]string{"owner": "platform"}
	labelsJSON, _ := json.Marshal(storedLabels)
	annotationsJSON, _ := json.Marshal(storedAnnotations)

	rollbackCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rollbackConfigMapName("victim-ns"),
			Namespace: "safe-ns",
			Labels:    safety.ChaosLabels(string(v1alpha1.NamespaceDeletion)),
		},
		Data: map[string]string{
			"labels":      string(labelsJSON),
			"annotations": string(annotationsJSON),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(safeNs, rollbackCM).
		Build()

	injector := NewNamespaceDeletionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.NamespaceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"namespace": "victim-ns",
		},
	}

	err := injector.Revert(ctx, spec, "safe-ns")
	require.NoError(t, err)

	// Verify namespace was recreated with stored metadata
	var ns corev1.Namespace
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "victim-ns"}, &ns)
	require.NoError(t, err)
	assert.Equal(t, "production", ns.Labels["env"])
	assert.Equal(t, "platform", ns.Annotations["owner"])

	// Verify rollback ConfigMap was deleted
	var cm corev1.ConfigMap
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      rollbackConfigMapName("victim-ns"),
		Namespace: "safe-ns",
	}, &cm)
	assert.True(t, apierrors.IsNotFound(err), "rollback ConfigMap should be deleted")
}

func TestNamespaceDeletionRevertNamespaceAlreadyExists(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	// Namespace already exists (operator recreated it)
	victimNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "victim-ns",
		},
	}

	safeNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "safe-ns",
		},
	}

	rollbackCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rollbackConfigMapName("victim-ns"),
			Namespace: "safe-ns",
			Labels:    safety.ChaosLabels(string(v1alpha1.NamespaceDeletion)),
		},
		Data: map[string]string{
			"labels":      "{}",
			"annotations": "{}",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(victimNs, safeNs, rollbackCM).
		Build()

	injector := NewNamespaceDeletionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.NamespaceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"namespace": "victim-ns",
		},
	}

	err := injector.Revert(ctx, spec, "safe-ns")
	require.NoError(t, err)

	// Verify namespace still exists (not recreated)
	var ns corev1.Namespace
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "victim-ns"}, &ns)
	require.NoError(t, err)

	// Verify rollback ConfigMap was deleted
	var cm corev1.ConfigMap
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      rollbackConfigMapName("victim-ns"),
		Namespace: "safe-ns",
	}, &cm)
	assert.True(t, apierrors.IsNotFound(err), "rollback ConfigMap should be deleted")
}

func TestNamespaceDeletionRevertNoRollbackConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	safeNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "safe-ns",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(safeNs).
		Build()

	injector := NewNamespaceDeletionInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.NamespaceDeletion,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"namespace": "victim-ns",
		},
	}

	// Revert should be no-op when no rollback ConfigMap exists
	err := injector.Revert(ctx, spec, "safe-ns")
	assert.NoError(t, err)
}

func TestNamespaceDeletionValidate(t *testing.T) {
	injector := &NamespaceDeletionInjector{}
	blast := v1alpha1.BlastRadiusSpec{
		MaxPodsAffected:   100,
		AllowedNamespaces: []string{"test"},
	}

	tests := []struct {
		name    string
		spec    v1alpha1.InjectionSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid spec",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.NamespaceDeletion,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"namespace": "test-ns",
				},
			},
			wantErr: false,
		},
		{
			name: "missing dangerLevel high",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.NamespaceDeletion,
				DangerLevel: v1alpha1.DangerLevelMedium,
				Parameters: map[string]string{
					"namespace": "test-ns",
				},
			},
			wantErr: true,
			errMsg:  "dangerLevel: high",
		},
		{
			name: "missing namespace param",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.NamespaceDeletion,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters:  map[string]string{},
			},
			wantErr: true,
			errMsg:  "namespace",
		},
		{
			name: "forbidden kube-system",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.NamespaceDeletion,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"namespace": "kube-system",
				},
			},
			wantErr: true,
			errMsg:  "kube-system",
		},
		{
			name: "forbidden default",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.NamespaceDeletion,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"namespace": "default",
				},
			},
			wantErr: true,
			errMsg:  "default",
		},
		{
			name: "forbidden openshift prefix",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.NamespaceDeletion,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"namespace": "openshift-monitoring",
				},
			},
			wantErr: true,
			errMsg:  "openshift-",
		},
		{
			name: "forbidden chaos prefix",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.NamespaceDeletion,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"namespace": "chaos-test",
				},
			},
			wantErr: true,
			errMsg:  "chaos-",
		},
		{
			name: "forbidden controller namespace",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.NamespaceDeletion,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"namespace": "odh-chaos-system",
				},
			},
			wantErr: true,
			errMsg:  "controller namespace",
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
