package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestConfigDriftValidate(t *testing.T) {
	injector := &ConfigDriftInjector{}
	blast := v1alpha1.BlastRadiusSpec{
		MaxPodsAffected:   1,
		AllowedNamespaces: []string{"test"},
	}

	tests := []struct {
		name    string
		spec    v1alpha1.InjectionSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid spec with all required params",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.ConfigDrift,
				Parameters: map[string]string{
					"name":  "my-configmap",
					"key":   "config.yaml",
					"value": "corrupted-data",
				},
			},
			wantErr: false,
		},
		{
			name: "valid spec with Secret resourceType",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.ConfigDrift,
				Parameters: map[string]string{
					"name":         "my-secret",
					"key":          "password",
					"value":        "wrong-password",
					"resourceType": "Secret",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.ConfigDrift,
				Parameters: map[string]string{
					"key":   "config.yaml",
					"value": "corrupted-data",
				},
			},
			wantErr: true,
			errMsg:  "name",
		},
		{
			name: "missing key",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.ConfigDrift,
				Parameters: map[string]string{
					"name":  "my-configmap",
					"value": "corrupted-data",
				},
			},
			wantErr: true,
			errMsg:  "key",
		},
		{
			name: "missing value",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.ConfigDrift,
				Parameters: map[string]string{
					"name": "my-configmap",
					"key":  "config.yaml",
				},
			},
			wantErr: true,
			errMsg:  "value",
		},
		{
			name: "nil parameters",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.ConfigDrift,
			},
			wantErr: true,
			errMsg:  "name",
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
