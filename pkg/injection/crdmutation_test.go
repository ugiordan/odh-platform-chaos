package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestCRDMutationValidate(t *testing.T) {
	injector := &CRDMutationInjector{}
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
				Type: v1alpha1.CRDMutation,
				Parameters: map[string]string{
					"apiVersion": "v1",
					"kind":       "DataScienceCluster",
					"name":       "default-dsc",
					"field":      "replicas",
					"value":      "0",
				},
			},
			wantErr: false,
		},
		{
			name: "missing apiVersion",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.CRDMutation,
				Parameters: map[string]string{
					"kind":  "DataScienceCluster",
					"name":  "default-dsc",
					"field": "replicas",
					"value": "0",
				},
			},
			wantErr: true,
			errMsg:  "apiVersion",
		},
		{
			name: "missing kind",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.CRDMutation,
				Parameters: map[string]string{
					"apiVersion": "v1",
					"name":       "default-dsc",
					"field":      "replicas",
					"value":      "0",
				},
			},
			wantErr: true,
			errMsg:  "kind",
		},
		{
			name: "missing name",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.CRDMutation,
				Parameters: map[string]string{
					"apiVersion": "v1",
					"kind":       "DataScienceCluster",
					"field":      "replicas",
					"value":      "0",
				},
			},
			wantErr: true,
			errMsg:  "name",
		},
		{
			name: "missing field",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.CRDMutation,
				Parameters: map[string]string{
					"apiVersion": "v1",
					"kind":       "DataScienceCluster",
					"name":       "default-dsc",
					"value":      "0",
				},
			},
			wantErr: true,
			errMsg:  "field",
		},
		{
			name: "missing value",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.CRDMutation,
				Parameters: map[string]string{
					"apiVersion": "v1",
					"kind":       "DataScienceCluster",
					"name":       "default-dsc",
					"field":      "replicas",
				},
			},
			wantErr: true,
			errMsg:  "value",
		},
		{
			name: "nil parameters",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.CRDMutation,
			},
			wantErr: true,
			errMsg:  "apiVersion",
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
