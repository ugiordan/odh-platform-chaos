package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RealKnowledgeFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	models, err := LoadKnowledgeDir("../../knowledge")
	require.NoError(t, err)
	require.NotEmpty(t, models)

	graph, err := BuildDependencyGraph(models)
	require.NoError(t, err)

	// Intra-operator: kserve-controller-manager faulted → llmisvc-controller-manager depends on it
	deps := graph.DirectDependents(ComponentRef{Operator: "kserve", Component: "kserve-controller-manager"})
	foundLlmisvc := false
	for _, d := range deps {
		if d.Component.Name == "llmisvc-controller-manager" {
			foundLlmisvc = true
			break
		}
	}
	assert.True(t, foundLlmisvc, "llmisvc-controller-manager should depend on kserve-controller-manager")

	// Cross-operator: odh-model-controller depends on "kserve" operator
	foundOdh := false
	for _, d := range deps {
		if d.Component.Name == "odh-model-controller" {
			foundOdh = true
			break
		}
	}
	assert.True(t, foundOdh, "odh-model-controller should appear as cross-operator dependent of kserve")
}
