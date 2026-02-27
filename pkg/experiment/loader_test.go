package experiment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadExperiment(t *testing.T) {
	exp, err := Load("../../testdata/experiments/valid-experiment.yaml")
	require.NoError(t, err)
	assert.Equal(t, "dashboard-pod-kill-recovery", exp.Metadata.Name)
}

func TestLoadExperimentFileNotFound(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	assert.Error(t, err)
}

func TestValidateExperiment(t *testing.T) {
	exp, err := Load("../../testdata/experiments/valid-experiment.yaml")
	require.NoError(t, err)

	errs := Validate(exp)
	assert.Empty(t, errs)
}

func TestValidateExperimentMissingFields(t *testing.T) {
	exp, err := Load("../../testdata/experiments/invalid-experiment.yaml")
	require.NoError(t, err)

	errs := Validate(exp)
	assert.NotEmpty(t, errs)
}

func TestValidateBlastRadius(t *testing.T) {
	exp, err := Load("../../testdata/experiments/valid-experiment.yaml")
	require.NoError(t, err)

	// Valid: maxPodsAffected > 0 and allowedNamespaces not empty
	errs := Validate(exp)
	assert.Empty(t, errs)

	// Invalid: no allowed namespaces
	exp.Spec.BlastRadius.AllowedNamespaces = nil
	errs = Validate(exp)
	assert.NotEmpty(t, errs)
}
