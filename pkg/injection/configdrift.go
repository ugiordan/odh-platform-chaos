package injection

import (
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/safety"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigDriftInjector struct {
	client client.Client
}

func NewConfigDriftInjector(c client.Client) *ConfigDriftInjector {
	return &ConfigDriftInjector{client: c}
}

func (d *ConfigDriftInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	if _, ok := spec.Parameters["name"]; !ok {
		return fmt.Errorf("ConfigDrift requires 'name' parameter")
	}
	if _, ok := spec.Parameters["key"]; !ok {
		return fmt.Errorf("ConfigDrift requires 'key' parameter (data key to modify)")
	}
	if _, ok := spec.Parameters["value"]; !ok {
		return fmt.Errorf("ConfigDrift requires 'value' parameter (corrupted value)")
	}
	// resourceType defaults to "ConfigMap"
	return nil
}

func (d *ConfigDriftInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	key := types.NamespacedName{
		Name:      spec.Parameters["name"],
		Namespace: namespace,
	}

	resourceType := spec.Parameters["resourceType"]
	if resourceType == "" {
		resourceType = "ConfigMap"
	}

	var originalValue string

	if resourceType == "Secret" {
		secret := &corev1.Secret{}
		if err := d.client.Get(ctx, key, secret); err != nil {
			return nil, nil, fmt.Errorf("getting Secret %s: %w", key, err)
		}
		dataKey := spec.Parameters["key"]
		originalValue = string(secret.Data[dataKey])
		secret.Data[dataKey] = []byte(spec.Parameters["value"])

		// Store rollback annotation for crash-safe recovery
		rollbackInfo := map[string]string{
			"resourceType":  "Secret",
			"key":           dataKey,
			"originalValue": originalValue,
		}
		rollbackJSON, err := json.Marshal(rollbackInfo)
		if err != nil {
			return nil, nil, fmt.Errorf("serializing rollback data for Secret %s: %w", key, err)
		}
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		secret.Annotations[safety.RollbackAnnotationKey] = string(rollbackJSON)

		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		for k, v := range safety.ChaosLabels(string(v1alpha1.ConfigDrift)) {
			secret.Labels[k] = v
		}

		if err := d.client.Update(ctx, secret); err != nil {
			return nil, nil, fmt.Errorf("updating Secret: %w", err)
		}
		cleanup := func(ctx context.Context) error {
			s := &corev1.Secret{}
			if err := d.client.Get(ctx, key, s); err != nil {
				return err
			}
			s.Data[dataKey] = []byte(originalValue)

			// Remove rollback annotation and chaos labels
			delete(s.Annotations, safety.RollbackAnnotationKey)
			for k := range safety.ChaosLabels(string(v1alpha1.ConfigDrift)) {
				delete(s.Labels, k)
			}

			return d.client.Update(ctx, s)
		}
		events := []v1alpha1.InjectionEvent{
			NewEvent(v1alpha1.ConfigDrift, key.String(), "drifted",
				map[string]string{"resourceType": "Secret", "key": dataKey}),
		}
		return cleanup, events, nil
	}

	// Default: ConfigMap
	cm := &corev1.ConfigMap{}
	if err := d.client.Get(ctx, key, cm); err != nil {
		return nil, nil, fmt.Errorf("getting ConfigMap %s: %w", key, err)
	}
	dataKey := spec.Parameters["key"]
	originalValue = cm.Data[dataKey]
	cm.Data[dataKey] = spec.Parameters["value"]

	// Store rollback annotation for crash-safe recovery
	rollbackInfo := map[string]string{
		"resourceType":  "ConfigMap",
		"key":           dataKey,
		"originalValue": originalValue,
	}
	rollbackJSON, err := json.Marshal(rollbackInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing rollback data for ConfigMap %s: %w", key, err)
	}
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	cm.Annotations[safety.RollbackAnnotationKey] = string(rollbackJSON)

	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	for k, v := range safety.ChaosLabels(string(v1alpha1.ConfigDrift)) {
		cm.Labels[k] = v
	}

	if err := d.client.Update(ctx, cm); err != nil {
		return nil, nil, fmt.Errorf("updating ConfigMap: %w", err)
	}

	cleanup := func(ctx context.Context) error {
		c := &corev1.ConfigMap{}
		if err := d.client.Get(ctx, key, c); err != nil {
			return err
		}
		c.Data[dataKey] = originalValue

		// Remove rollback annotation and chaos labels
		delete(c.Annotations, safety.RollbackAnnotationKey)
		for k := range safety.ChaosLabels(string(v1alpha1.ConfigDrift)) {
			delete(c.Labels, k)
		}

		return d.client.Update(ctx, c)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.ConfigDrift, key.String(), "drifted",
			map[string]string{"resourceType": "ConfigMap", "key": dataKey}),
	}

	return cleanup, events, nil
}
