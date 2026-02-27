package safety

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestValidateBlastRadius(t *testing.T) {
	tests := []struct {
		name    string
		spec    v1alpha1.BlastRadiusSpec
		target  string
		wantErr bool
	}{
		{
			name: "valid blast radius",
			spec: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:   1,
				AllowedNamespaces: []string{"opendatahub"},
			},
			target:  "opendatahub",
			wantErr: false,
		},
		{
			name: "namespace not allowed",
			spec: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:   1,
				AllowedNamespaces: []string{"opendatahub"},
			},
			target:  "kube-system",
			wantErr: true,
		},
		{
			name: "zero pods allowed",
			spec: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:   0,
				AllowedNamespaces: []string{"opendatahub"},
			},
			target:  "opendatahub",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBlastRadius(tt.spec, tt.target, 1)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckDangerLevel(t *testing.T) {
	err := CheckDangerLevel("high", false)
	assert.Error(t, err)

	err = CheckDangerLevel("high", true)
	assert.NoError(t, err)

	err = CheckDangerLevel("", false)
	assert.NoError(t, err)
}
