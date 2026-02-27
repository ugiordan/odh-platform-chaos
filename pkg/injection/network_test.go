package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestNetworkPartitionValidate(t *testing.T) {
	injector := &NetworkPartitionInjector{}
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
			name: "valid spec with labelSelector",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.NetworkPartition,
				Parameters: map[string]string{
					"labelSelector": "app=dashboard",
				},
			},
			wantErr: false,
		},
		{
			name: "missing labelSelector",
			spec: v1alpha1.InjectionSpec{
				Type:       v1alpha1.NetworkPartition,
				Parameters: map[string]string{},
			},
			wantErr: true,
			errMsg:  "labelSelector",
		},
		{
			name: "nil parameters",
			spec: v1alpha1.InjectionSpec{
				Type: v1alpha1.NetworkPartition,
			},
			wantErr: true,
			errMsg:  "labelSelector",
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
