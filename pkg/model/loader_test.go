package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadKnowledgeDir(t *testing.T) {
	models, err := LoadKnowledgeDir("../../testdata/knowledge")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(models), 2)

	names := make(map[string]bool)
	for _, m := range models {
		names[m.Operator.Name] = true
	}
	assert.True(t, names["alpha-operator"])
	assert.True(t, names["beta-operator"])
}

func TestLoadKnowledgeDir_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()
	validYAML := `operator:
  name: test
  namespace: test-ns
components: []
recovery:
  reconcileTimeout: "60s"
  maxReconcileCycles: 5`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte(validYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0644))

	models, err := LoadKnowledgeDir(dir)
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "test", models[0].Operator.Name)
}

func TestLoadKnowledgeDir_InvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("not: valid: yaml: ["), 0644))

	_, err := LoadKnowledgeDir(dir)
	assert.Error(t, err)
}

func TestLoadKnowledgeDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	models, err := LoadKnowledgeDir(dir)
	require.NoError(t, err)
	assert.Empty(t, models)
}
