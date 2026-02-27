package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestPodKillValidate(t *testing.T) {
	injector := &PodKillInjector{}

	// Valid spec
	spec := v1alpha1.InjectionSpec{
		Type:  v1alpha1.PodKill,
		Count: 1,
		Parameters: map[string]string{
			"labelSelector": "app=dashboard",
		},
	}
	blast := v1alpha1.BlastRadiusSpec{
		MaxPodsAffected:   1,
		AllowedNamespaces: []string{"test"},
	}

	err := injector.Validate(spec, blast)
	assert.NoError(t, err)

	// Invalid: count exceeds blast radius
	spec.Count = 5
	err = injector.Validate(spec, blast)
	assert.Error(t, err)
}

func TestPodKillValidateMissingSelector(t *testing.T) {
	injector := &PodKillInjector{}

	spec := v1alpha1.InjectionSpec{
		Type:  v1alpha1.PodKill,
		Count: 1,
	}
	blast := v1alpha1.BlastRadiusSpec{
		MaxPodsAffected:   1,
		AllowedNamespaces: []string{"test"},
	}

	err := injector.Validate(spec, blast)
	assert.Error(t, err)
}
