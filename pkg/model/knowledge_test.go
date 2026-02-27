package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadKnowledge(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	assert.Equal(t, "test-operator", k.Operator.Name)
	assert.Equal(t, "test-ns", k.Operator.Namespace)
	assert.Len(t, k.Components, 2)
}

func TestGetComponent(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	comp := k.GetComponent("dashboard")
	require.NotNil(t, comp)
	assert.Equal(t, "dashboard", comp.Name)
	assert.Len(t, comp.ManagedResources, 2)
	assert.Equal(t, "Deployment", comp.ManagedResources[0].Kind)
}

func TestGetComponentNotFound(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	comp := k.GetComponent("nonexistent")
	assert.Nil(t, comp)
}

func TestKnowledgeRecoveryDefaults(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	assert.Equal(t, 300*time.Second, k.Recovery.ReconcileTimeout.Duration)
	assert.Equal(t, 10, k.Recovery.MaxReconcileCycles)
}

func TestManagedResourceExpectedSpec(t *testing.T) {
	k, err := LoadKnowledge("../../testdata/knowledge/test-operator.yaml")
	require.NoError(t, err)

	comp := k.GetComponent("dashboard")
	require.NotNil(t, comp)

	deploy := comp.ManagedResources[0]
	assert.Equal(t, "Deployment", deploy.Kind)
	assert.NotNil(t, deploy.ExpectedSpec)
}
