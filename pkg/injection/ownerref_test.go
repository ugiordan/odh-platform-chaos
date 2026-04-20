package injection

import (
	"context"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestOwnerRefOrphanInjectAndCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	isController := true
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("my-deploy")
	obj.SetNamespace("test-ns")
	obj.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "custom.example.com/v1",
			Kind:       "MyOperator",
			Name:       "my-operator-instance",
			UID:        "abc-123",
			Controller: &isController,
		},
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewOwnerRefOrphanInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.OwnerRefOrphan,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "my-deploy",
		},
	}

	// Inject
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	require.Len(t, events, 1)
	assert.Equal(t, "orphaned", events[0].Action)

	// Verify ownerReferences were removed
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("my-deploy", "test-ns"), current))
	assert.Empty(t, current.GetOwnerReferences())

	// Verify rollback annotation exists
	annotations := current.GetAnnotations()
	require.NotNil(t, annotations)
	_, hasRollback := annotations[safety.RollbackAnnotationKey]
	assert.True(t, hasRollback)

	// Verify chaos labels
	labels := current.GetLabels()
	assert.Equal(t, safety.ManagedByValue, labels[safety.ManagedByLabel])

	// Cleanup restores ownerReferences
	require.NoError(t, cleanup(ctx))

	restored := &unstructured.Unstructured{}
	restored.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("my-deploy", "test-ns"), restored))
	ownerRefs := restored.GetOwnerReferences()
	require.Len(t, ownerRefs, 1)
	assert.Equal(t, "MyOperator", ownerRefs[0].Kind)
	assert.Equal(t, "my-operator-instance", ownerRefs[0].Name)

	// Rollback annotation should be removed
	restoredAnnotations := restored.GetAnnotations()
	if restoredAnnotations != nil {
		_, hasAnnotation := restoredAnnotations[safety.RollbackAnnotationKey]
		assert.False(t, hasAnnotation)
	}
}

func TestOwnerRefOrphanInjectNoOwnerRefs(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("no-owner-deploy")
	obj.SetNamespace("test-ns")

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewOwnerRefOrphanInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.OwnerRefOrphan,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "no-owner-deploy",
		},
	}

	_, _, err := injector.Inject(ctx, spec, "test-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no ownerReferences")
}

func TestOwnerRefOrphanCleanupSkipsIfReAdopted(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	isController := true
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("readopted-deploy")
	obj.SetNamespace("test-ns")
	obj.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "custom.example.com/v1",
			Kind:       "MyOperator",
			Name:       "my-operator",
			UID:        "uid-1",
			Controller: &isController,
		},
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewOwnerRefOrphanInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.OwnerRefOrphan,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "readopted-deploy",
		},
	}

	cleanup, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Simulate operator re-adoption before cleanup runs
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("readopted-deploy", "test-ns"), current))
	current.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "custom.example.com/v1",
			Kind:       "MyOperator",
			Name:       "my-operator",
			UID:        "uid-1",
			Controller: &isController,
		},
	})
	require.NoError(t, fakeClient.Update(ctx, current))

	// Cleanup should just remove chaos metadata, not overwrite ownerRefs
	require.NoError(t, cleanup(ctx))

	restored := &unstructured.Unstructured{}
	restored.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("readopted-deploy", "test-ns"), restored))
	ownerRefs := restored.GetOwnerReferences()
	require.Len(t, ownerRefs, 1)
	assert.Equal(t, "my-operator", ownerRefs[0].Name)
}

func TestOwnerRefOrphanRevert(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	isController := true
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("revert-deploy")
	obj.SetNamespace("test-ns")
	obj.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "custom.example.com/v1",
			Kind:       "MyOperator",
			Name:       "my-operator",
			UID:        "uid-revert",
			Controller: &isController,
		},
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewOwnerRefOrphanInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.OwnerRefOrphan,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "revert-deploy",
		},
	}

	// Inject then Revert
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	err = injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	restored := &unstructured.Unstructured{}
	restored.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("revert-deploy", "test-ns"), restored))
	ownerRefs := restored.GetOwnerReferences()
	require.Len(t, ownerRefs, 1)
	assert.Equal(t, "my-operator", ownerRefs[0].Name)

	// Idempotent
	err = injector.Revert(ctx, spec, "test-ns")
	assert.NoError(t, err)
}

func TestOwnerRefOrphanValidate(t *testing.T) {
	injector := &OwnerRefOrphanInjector{}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	tests := []struct {
		name    string
		spec    v1alpha1.InjectionSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid spec",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.OwnerRefOrphan,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
				},
			},
			wantErr: false,
		},
		{
			name: "missing apiVersion",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.OwnerRefOrphan,
				Parameters: map[string]string{
					"kind": "Deployment",
					"name": "my-deploy",
				},
			},
			wantErr: true,
			errMsg:  "apiVersion",
		},
		{
			name: "missing kind",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.OwnerRefOrphan,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"name":       "my-deploy",
				},
			},
			wantErr: true,
			errMsg:  "kind",
		},
		{
			name: "missing name",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.OwnerRefOrphan,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			wantErr: true,
			errMsg:  "name",
		},
		{
			name: "forbidden Namespace kind",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.OwnerRefOrphan,
				Parameters: map[string]string{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"name":       "test-ns",
				},
			},
			wantErr: true,
			errMsg:  "not allowed",
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
