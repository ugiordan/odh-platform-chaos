package injection

import (
	"testing"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestRegistryLookup(t *testing.T) {
	r := NewRegistry()
	r.Register(v1alpha1.PodKill, &PodKillInjector{})

	injector, err := r.Get(v1alpha1.PodKill)
	assert.NoError(t, err)
	assert.NotNil(t, injector)

	_, err = r.Get("UnknownType")
	assert.Error(t, err)
}

func TestRegistryListTypes(t *testing.T) {
	r := NewRegistry()
	r.Register(v1alpha1.PodKill, &PodKillInjector{})

	types := r.ListTypes()
	assert.Contains(t, types, v1alpha1.PodKill)
}
