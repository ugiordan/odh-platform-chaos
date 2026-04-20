package injection

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	webhookLatencyImage      = "registry.k8s.io/e2e-test-images/agnhost:2.40"
	webhookLatencyPort       = 8443
	webhookLatencyNamePrefix = "chaos-webhook-latency-"
	defaultWebhookDelay      = 25 * time.Second
)

// WebhookLatencyInjector deploys a slow admission webhook that adds configurable
// latency to API server requests for specific resource types. This tests whether
// operators handle slow API responses without hanging or timing out.
type WebhookLatencyInjector struct {
	client client.Client
}

// NewWebhookLatencyInjector creates a new WebhookLatencyInjector.
func NewWebhookLatencyInjector(c client.Client) *WebhookLatencyInjector {
	return &WebhookLatencyInjector{client: c}
}

func (w *WebhookLatencyInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateWebhookLatencyParams(spec)
}

// Inject creates a Deployment running a slow webhook server, a Service, and a
// ValidatingWebhookConfiguration that intercepts the specified resource types.
func (w *WebhookLatencyInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	resources := strings.Split(spec.Parameters["resources"], ",")
	apiGroups := strings.Split(spec.Parameters["apiGroups"], ",")

	delay := defaultWebhookDelay
	if d := spec.Parameters["delay"]; d != "" {
		var err error
		delay, err = time.ParseDuration(d)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing delay: %w", err)
		}
	}

	name := webhookLatencyNamePrefix + sanitizeForK8s(spec.Parameters["resources"])
	chaosLabels := safety.ChaosLabels(string(v1alpha1.WebhookLatency))
	chaosLabels["app"] = name

	// Check if resources already exist
	existingDeploy := &appsv1.Deployment{}
	if err := w.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existingDeploy); err == nil {
		return nil, nil, fmt.Errorf("WebhookLatency deployment %q already exists; clean up before re-injecting", name)
	}

	replicas := int32(1)
	sideEffect := admissionregistrationv1.SideEffectClassNone
	failPolicy := admissionregistrationv1.Ignore
	timeoutSec := int32(30)

	// Create the webhook server Deployment using agnhost's webhook server
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    chaosLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "webhook",
							Image: webhookLatencyImage,
							Args: []string{
								"webhook",
								fmt.Sprintf("--delay=%s", delay),
								fmt.Sprintf("--port=%d", webhookLatencyPort),
								"--tls-cert-file=/etc/webhook/certs/tls.crt",
								"--tls-private-key-file=/etc/webhook/certs/tls.key",
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: int32(webhookLatencyPort), Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "certs", MountPath: "/etc/webhook/certs", ReadOnly: true},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: name + "-certs",
								},
							},
						},
					},
				},
			},
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    chaosLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": name},
			Ports: []corev1.ServicePort{
				{Port: 443, TargetPort: intstr.FromInt(webhookLatencyPort), Protocol: corev1.ProtocolTCP},
			},
		},
	}

	webhookPath := "/validate"
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: chaosLabels,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: name + ".chaos.operatorchaos.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      name,
						Namespace: namespace,
						Path:      &webhookPath,
					},
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   apiGroups,
							APIVersions: []string{"*"},
							Resources:   resources,
						},
					},
				},
				SideEffects:             &sideEffect,
				FailurePolicy:           &failPolicy,
				TimeoutSeconds:          &timeoutSec,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	// Generate self-signed TLS certificate for the webhook server
	certPEM, keyPEM, caPEM, err := generateSelfSignedCert(name, namespace)
	if err != nil {
		return nil, nil, fmt.Errorf("generating TLS certificate: %w", err)
	}

	// Create TLS Secret
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-certs",
			Namespace: namespace,
			Labels:    chaosLabels,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certPEM,
			corev1.TLSPrivateKeyKey: keyPEM,
		},
	}

	// Set CA bundle on webhook configuration so API server trusts our self-signed cert
	webhookConfig.Webhooks[0].ClientConfig.CABundle = caPEM

	// Create resources in order: Secret, Deployment, Service, WebhookConfiguration
	for _, obj := range []client.Object{tlsSecret, deploy, svc, webhookConfig} {
		if err := w.client.Create(ctx, obj); err != nil {
			// Best-effort cleanup of already-created resources
			_ = w.cleanupResources(ctx, name, namespace)
			return nil, nil, fmt.Errorf("creating %T %q: %w", obj, name, err)
		}
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.WebhookLatency, namespace+"/"+name, "deployed",
			map[string]string{
				"delay":     delay.String(),
				"resources": spec.Parameters["resources"],
				"apiGroups": spec.Parameters["apiGroups"],
			}),
	}

	cleanup := func(ctx context.Context) error {
		return w.cleanupResources(ctx, name, namespace)
	}

	return cleanup, events, nil
}

// Revert removes the webhook deployment, service, and configuration.
func (w *WebhookLatencyInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	name := webhookLatencyNamePrefix + sanitizeForK8s(spec.Parameters["resources"])
	return w.cleanupResources(ctx, name, namespace)
}

// cleanupResources removes the Secret, Deployment, Service, and ValidatingWebhookConfiguration.
func (w *WebhookLatencyInjector) cleanupResources(ctx context.Context, name, namespace string) error {
	var errs []string

	// Delete webhook config first (cluster-scoped)
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	if err := w.client.Delete(ctx, webhookConfig); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("deleting webhook config: %v", err))
	}

	// Delete service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	if err := w.client.Delete(ctx, svc); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("deleting service: %v", err))
	}

	// Delete deployment
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	if err := w.client.Delete(ctx, deploy); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("deleting deployment: %v", err))
	}

	// Delete TLS Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-certs", Namespace: namespace},
	}
	if err := w.client.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("deleting TLS secret: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// generateSelfSignedCert creates a self-signed CA and server certificate for
// the webhook server. Returns PEM-encoded cert, key, and CA cert.
func generateSelfSignedCert(name, namespace string) (certPEM, keyPEM, caPEM []byte, err error) {
	// Generate CA key
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generating CA key: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "chaos-webhook-ca"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating CA certificate: %w", err)
	}
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	// Generate server key
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generating server key: %w", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: name + "." + namespace + ".svc"},
		DNSNames: []string{
			name,
			name + "." + namespace,
			name + "." + namespace + ".svc",
			name + "." + namespace + ".svc.cluster.local",
		},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating server certificate: %w", err)
	}

	// Encode to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})
	serverKeyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshaling server key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: serverKeyDER})
	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	return certPEM, keyPEM, caPEM, nil
}

// sanitizeForK8s converts a comma-separated resource list into a valid K8s name suffix.
func sanitizeForK8s(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ",", "-")
	s = strings.ReplaceAll(s, " ", "")
	if len(s) > 30 {
		s = s[:30]
	}
	s = strings.TrimRight(s, "-")
	if s == "" {
		s = "default"
	}
	return s
}
