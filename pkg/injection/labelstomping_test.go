package injection

import (
	"context"
	"strings"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLabelStompingInjectOverwrite(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("my-deploy")
	obj.SetNamespace("test-ns")
	obj.SetLabels(map[string]string{
		"app.kubernetes.io/name": "my-app",
		"version":                "v1",
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewLabelStompingInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.LabelStomping,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "my-deploy",
			"labelKey":   "app.kubernetes.io/name",
			"action":     "overwrite",
		},
	}

	// Inject
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	require.Len(t, events, 1)
	assert.Equal(t, "label-stomped", events[0].Action)
	assert.Equal(t, "app.kubernetes.io/name", events[0].Details["labelKey"])
	assert.Equal(t, "overwrite", events[0].Details["action"])
	assert.Equal(t, "my-app", events[0].Details["originalValue"])
	assert.Equal(t, "chaos-stomped", events[0].Details["newValue"])

	// Verify label was overwritten
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("my-deploy", "test-ns"), current))
	labels := current.GetLabels()
	assert.Equal(t, "chaos-stomped", labels["app.kubernetes.io/name"])
	assert.Equal(t, "v1", labels["version"]) // other labels unchanged

	// Verify rollback annotation exists
	annotations := current.GetAnnotations()
	require.NotNil(t, annotations)
	rollbackStr, ok := annotations[safety.RollbackAnnotationKey]
	require.True(t, ok)

	var rollbackInfo map[string]any
	require.NoError(t, safety.UnwrapRollbackData(rollbackStr, &rollbackInfo))
	assert.Equal(t, "app.kubernetes.io/name", rollbackInfo["labelKey"])
	assert.Equal(t, "my-app", rollbackInfo["originalValue"])
	assert.Equal(t, true, rollbackInfo["existed"])

	// Verify chaos labels applied
	assert.Equal(t, safety.ManagedByValue, labels[safety.ManagedByLabel])
	assert.Equal(t, string(v1alpha1.LabelStomping), labels[safety.ChaosTypeLabel])

	// Cleanup restores original
	require.NoError(t, cleanup(ctx))

	restored := &unstructured.Unstructured{}
	restored.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("my-deploy", "test-ns"), restored))
	restoredLabels := restored.GetLabels()
	assert.Equal(t, "my-app", restoredLabels["app.kubernetes.io/name"])
	assert.Equal(t, "v1", restoredLabels["version"])

	// Chaos labels removed
	_, hasManagedBy := restoredLabels[safety.ManagedByLabel]
	assert.False(t, hasManagedBy)
	_, hasChaosType := restoredLabels[safety.ChaosTypeLabel]
	assert.False(t, hasChaosType)

	// Rollback annotation removed
	restoredAnnotations := restored.GetAnnotations()
	if restoredAnnotations != nil {
		_, hasRollback := restoredAnnotations[safety.RollbackAnnotationKey]
		assert.False(t, hasRollback)
	}
}

func TestLabelStompingInjectOverwriteCustomValue(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("my-deploy")
	obj.SetNamespace("test-ns")
	obj.SetLabels(map[string]string{
		"app.kubernetes.io/name": "my-app",
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewLabelStompingInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.LabelStomping,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "my-deploy",
			"labelKey":   "app.kubernetes.io/name",
			"action":     "overwrite",
			"newValue":   "custom-chaos-value",
		},
	}

	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "custom-chaos-value", events[0].Details["newValue"])

	// Verify custom value used
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("my-deploy", "test-ns"), current))
	labels := current.GetLabels()
	assert.Equal(t, "custom-chaos-value", labels["app.kubernetes.io/name"])

	// Cleanup restores original
	require.NoError(t, cleanup(ctx))
	restored := &unstructured.Unstructured{}
	restored.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("my-deploy", "test-ns"), restored))
	restoredLabels := restored.GetLabels()
	assert.Equal(t, "my-app", restoredLabels["app.kubernetes.io/name"])
}

func TestLabelStompingInjectDelete(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("my-deploy")
	obj.SetNamespace("test-ns")
	obj.SetLabels(map[string]string{
		"app.kubernetes.io/name": "my-app",
		"version":                "v1",
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewLabelStompingInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.LabelStomping,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "my-deploy",
			"labelKey":   "app.kubernetes.io/name",
			"action":     "delete",
		},
	}

	// Inject
	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	require.Len(t, events, 1)
	assert.Equal(t, "delete", events[0].Details["action"])
	assert.Equal(t, "my-app", events[0].Details["originalValue"])
	assert.Equal(t, "<deleted>", events[0].Details["newValue"])

	// Verify label was removed
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("my-deploy", "test-ns"), current))
	labels := current.GetLabels()
	_, exists := labels["app.kubernetes.io/name"]
	assert.False(t, exists)
	assert.Equal(t, "v1", labels["version"]) // other labels unchanged

	// Cleanup restores label
	require.NoError(t, cleanup(ctx))

	restored := &unstructured.Unstructured{}
	restored.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("my-deploy", "test-ns"), restored))
	restoredLabels := restored.GetLabels()
	assert.Equal(t, "my-app", restoredLabels["app.kubernetes.io/name"])
	assert.Equal(t, "v1", restoredLabels["version"])
}

func TestLabelStompingInjectResourceNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewLabelStompingInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.LabelStomping,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "nonexistent",
			"labelKey":   "app",
			"action":     "overwrite",
		},
	}

	_, _, err := injector.Inject(ctx, spec, "test-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting resource")
}

func TestLabelStompingRevert(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("revert-deploy")
	obj.SetNamespace("test-ns")
	obj.SetLabels(map[string]string{
		"app": "original",
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewLabelStompingInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.LabelStomping,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "revert-deploy",
			"labelKey":   "app",
			"action":     "overwrite",
		},
	}

	// Inject then Revert
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	err = injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Verify label restored
	restored := &unstructured.Unstructured{}
	restored.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("revert-deploy", "test-ns"), restored))
	restoredLabels := restored.GetLabels()
	assert.Equal(t, "original", restoredLabels["app"])

	// Rollback annotation removed
	restoredAnnotations := restored.GetAnnotations()
	if restoredAnnotations != nil {
		_, hasRollback := restoredAnnotations[safety.RollbackAnnotationKey]
		assert.False(t, hasRollback)
	}

	// Second Revert is no-op
	err = injector.Revert(ctx, spec, "test-ns")
	assert.NoError(t, err)
}

func TestLabelStompingRevertDeleteAction(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("delete-revert")
	obj.SetNamespace("test-ns")
	obj.SetLabels(map[string]string{
		"app": "important",
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewLabelStompingInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.LabelStomping,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "delete-revert",
			"labelKey":   "app",
			"action":     "delete",
		},
	}

	// Inject (delete label)
	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Verify label was removed
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("delete-revert", "test-ns"), current))
	labels := current.GetLabels()
	_, exists := labels["app"]
	assert.False(t, exists)

	// Revert restores deleted label
	err = injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	restored := &unstructured.Unstructured{}
	restored.SetGroupVersionKind(gvk)
	require.NoError(t, fakeClient.Get(ctx, client_key("delete-revert", "test-ns"), restored))
	restoredLabels := restored.GetLabels()
	assert.Equal(t, "important", restoredLabels["app"])
}

func TestLabelStompingInjectDeleteNonExistentLabel(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		&unstructured.UnstructuredList{},
	)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("my-deploy")
	obj.SetNamespace("test-ns")
	obj.SetLabels(map[string]string{
		"existing-label": "value",
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	injector := NewLabelStompingInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.LabelStomping,
		Parameters: map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "my-deploy",
			"labelKey":   "nonexistent-label",
			"action":     "delete",
		},
	}

	_, _, err := injector.Inject(ctx, spec, "test-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	assert.Contains(t, err.Error(), "nonexistent-label")
}

func TestValidateLabelKeyEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
		errMsg  string
	}{
		{name: "simple key", key: "app", wantErr: false},
		{name: "key with dots", key: "app.kubernetes.io", wantErr: false},
		{name: "prefixed key", key: "example.com/name", wantErr: false},
		{name: "empty key", key: "", wantErr: true, errMsg: "must not be empty"},
		{name: "empty prefix before slash", key: "/name", wantErr: true, errMsg: "prefix before '/' must not be empty"},
		{name: "empty name after slash", key: "example.com/", wantErr: true, errMsg: "name portion must not be empty"},
		{name: "prefix too long", key: strings.Repeat("a", 254) + "/name", wantErr: true, errMsg: "prefix exceeds 253"},
		{name: "name too long", key: strings.Repeat("a", 64), wantErr: true, errMsg: "name portion exceeds 63"},
		{name: "invalid chars in name", key: "app!name", wantErr: true, errMsg: "not valid"},
		{name: "uppercase prefix rejected", key: "Example.Com/name", wantErr: true, errMsg: "not a valid DNS subdomain"},
		{name: "key with multiple slashes uses last", key: "example.com/sub/name", wantErr: true, errMsg: "not a valid DNS subdomain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLabelKey(tt.key)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLabelValueEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		errMsg  string
	}{
		{name: "empty value (valid)", value: "", wantErr: false},
		{name: "simple value", value: "v1", wantErr: false},
		{name: "value with dots and dashes", value: "my-app.v1", wantErr: false},
		{name: "value at max length", value: strings.Repeat("a", 63), wantErr: false},
		{name: "value exceeds max length", value: strings.Repeat("a", 64), wantErr: true, errMsg: "exceeds 63"},
		{name: "value with invalid chars", value: "hello world", wantErr: true, errMsg: "not a valid label value"},
		{name: "value starting with dash", value: "-invalid", wantErr: true, errMsg: "not a valid label value"},
		{name: "value ending with dash", value: "invalid-", wantErr: true, errMsg: "not a valid label value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLabelValue(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLabelStompingValidate(t *testing.T) {
	injector := &LabelStompingInjector{}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1}

	tests := []struct {
		name    string
		spec    v1alpha1.InjectionSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid overwrite",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
					"labelKey":   "app",
					"action":     "overwrite",
				},
			},
			wantErr: false,
		},
		{
			name: "valid delete",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
					"labelKey":   "version",
					"action":     "delete",
				},
			},
			wantErr: false,
		},
		{
			name: "missing apiVersion",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"kind":     "Deployment",
					"name":     "my-deploy",
					"labelKey": "app",
					"action":   "overwrite",
				},
			},
			wantErr: true,
			errMsg:  "apiVersion",
		},
		{
			name: "missing kind",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"name":       "my-deploy",
					"labelKey":   "app",
					"action":     "overwrite",
				},
			},
			wantErr: true,
			errMsg:  "kind",
		},
		{
			name: "missing name",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"labelKey":   "app",
					"action":     "overwrite",
				},
			},
			wantErr: true,
			errMsg:  "name",
		},
		{
			name: "missing labelKey",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
					"action":     "overwrite",
				},
			},
			wantErr: true,
			errMsg:  "labelKey",
		},
		{
			name: "invalid action",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
					"labelKey":   "app",
					"action":     "invalid",
				},
			},
			wantErr: true,
			errMsg:  "action must be 'overwrite' or 'delete'",
		},
		{
			name: "chaos-owned managed-by label",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
					"labelKey":   "app.kubernetes.io/managed-by",
					"action":     "overwrite",
				},
			},
			wantErr: true,
			errMsg:  "chaos-owned label",
		},
		{
			name: "chaos-owned prefix label",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
					"labelKey":   "chaos.operatorchaos.io/type",
					"action":     "overwrite",
				},
			},
			wantErr: true,
			errMsg:  "chaos-owned label",
		},
		{
			name: "system label without high danger",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.LabelStomping,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
					"labelKey":   "kubernetes.io/cluster-service",
					"action":     "overwrite",
				},
			},
			wantErr: true,
			errMsg:  "requires dangerLevel: high",
		},
		{
			name: "system label with high danger (allowed)",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.LabelStomping,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
					"labelKey":   "kubernetes.io/cluster-service",
					"action":     "overwrite",
				},
			},
			wantErr: false,
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
