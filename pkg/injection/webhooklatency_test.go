package injection

import (
	"context"
	"testing"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWebhookLatencyInjectAndCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, admissionregistrationv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewWebhookLatencyInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.WebhookLatency,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"resources": "deployments",
			"apiGroups": "apps",
			"delay":     "20s",
		},
	}

	cleanup, events, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	require.Len(t, events, 1)
	assert.Equal(t, "deployed", events[0].Action)

	expectedName := webhookLatencyNamePrefix + "deployments"

	// Verify TLS Secret was created
	secret := &corev1.Secret{}
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: expectedName + "-certs", Namespace: "test-ns"}, secret))
	assert.Equal(t, corev1.SecretTypeTLS, secret.Type)
	assert.NotEmpty(t, secret.Data[corev1.TLSCertKey])
	assert.NotEmpty(t, secret.Data[corev1.TLSPrivateKeyKey])

	// Verify Deployment was created
	deploy := &appsv1.Deployment{}
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: expectedName, Namespace: "test-ns"}, deploy))
	assert.Equal(t, safety.ManagedByValue, deploy.Labels[safety.ManagedByLabel])

	// Verify Service was created
	svc := &corev1.Service{}
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: expectedName, Namespace: "test-ns"}, svc))

	// Verify webhook config was created
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: expectedName}, webhookConfig))
	require.Len(t, webhookConfig.Webhooks, 1)
	require.Len(t, webhookConfig.Webhooks[0].Rules, 1)
	assert.Equal(t, []string{"apps"}, webhookConfig.Webhooks[0].Rules[0].APIGroups)
	assert.Equal(t, []string{"deployments"}, webhookConfig.Webhooks[0].Rules[0].Resources)
	assert.NotEmpty(t, webhookConfig.Webhooks[0].ClientConfig.CABundle, "CA bundle should be set")

	// Cleanup removes all resources
	require.NoError(t, cleanup(ctx))

	err = fakeClient.Get(ctx, types.NamespacedName{Name: expectedName, Namespace: "test-ns"}, deploy)
	assert.Error(t, err) // NotFound

	err = fakeClient.Get(ctx, types.NamespacedName{Name: expectedName, Namespace: "test-ns"}, svc)
	assert.Error(t, err) // NotFound

	err = fakeClient.Get(ctx, types.NamespacedName{Name: expectedName}, webhookConfig)
	assert.Error(t, err) // NotFound

	err = fakeClient.Get(ctx, types.NamespacedName{Name: expectedName + "-certs", Namespace: "test-ns"}, secret)
	assert.Error(t, err) // NotFound
}

func TestWebhookLatencyRevert(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, admissionregistrationv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewWebhookLatencyInjector(fakeClient)
	ctx := context.Background()

	spec := v1alpha1.InjectionSpec{
		Type:        v1alpha1.WebhookLatency,
		DangerLevel: v1alpha1.DangerLevelHigh,
		Parameters: map[string]string{
			"resources": "services",
			"apiGroups": "",
			"delay":     "15s",
		},
	}

	_, _, err := injector.Inject(ctx, spec, "test-ns")
	require.NoError(t, err)

	err = injector.Revert(ctx, spec, "test-ns")
	require.NoError(t, err)

	// Idempotent
	err = injector.Revert(ctx, spec, "test-ns")
	assert.NoError(t, err)
}

func TestWebhookLatencyValidate(t *testing.T) {
	injector := &WebhookLatencyInjector{}
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
				Type:        v1alpha1.WebhookLatency,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"resources": "deployments",
					"apiGroups": "apps",
					"delay":     "25s",
				},
			},
			wantErr: false,
		},
		{
			name: "missing dangerLevel high",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.WebhookLatency,
				Parameters: map[string]string{
					"resources": "deployments",
					"apiGroups": "apps",
				},
			},
			wantErr: true,
			errMsg:  "dangerLevel: high",
		},
		{
			name: "missing resources",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.WebhookLatency,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"apiGroups": "apps",
				},
			},
			wantErr: true,
			errMsg:  "resources",
		},
		{
			name: "missing apiGroups",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.WebhookLatency,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"resources": "deployments",
				},
			},
			wantErr: true,
			errMsg:  "apiGroups",
		},
		{
			name: "delay too high",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.WebhookLatency,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"resources": "deployments",
					"apiGroups": "apps",
					"delay":     "30s",
				},
			},
			wantErr: true,
			errMsg:  "exceeds 29s",
		},
		{
			name: "delay too short",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.WebhookLatency,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"resources": "deployments",
					"apiGroups": "apps",
					"delay":     "500ms",
				},
			},
			wantErr: true,
			errMsg:  "too short",
		},
		{
			name: "default delay (no delay param)",
			spec: v1alpha1.InjectionSpec{
				Type:        v1alpha1.WebhookLatency,
				DangerLevel: v1alpha1.DangerLevelHigh,
				Parameters: map[string]string{
					"resources": "deployments",
					"apiGroups": "apps",
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

func TestSanitizeForK8s(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"deployments", "deployments"},
		{"deployments,services", "deployments-services"},
		{"Deployments, Services", "deployments-services"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeForK8s(tt.input))
		})
	}
}
