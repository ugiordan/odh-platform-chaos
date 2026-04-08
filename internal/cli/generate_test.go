package cli

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCommand(t *testing.T) {
	k, err := model.LoadKnowledge("../../knowledge/dashboard.yaml")
	require.NoError(t, err)

	output, err := renderFuzzTargets(k)
	require.NoError(t, err)

	// Must parse as valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "fuzz_test.go", output, parser.AllErrors)
	require.NoError(t, err, "generated output is not valid Go:\n%s", output)

	// Must contain expected elements
	assert.Contains(t, output, "package fuzz_test")
	assert.Contains(t, output, "reconcilerFactory")
	assert.Contains(t, output, "FuzzOdhDashboard")
	assert.Contains(t, output, `// Code generated from knowledge model: dashboard. DO NOT EDIT`)
	assert.Contains(t, output, "seeds...")
	assert.Contains(t, output, "f.Add(")
}

func TestGenerateCommandMultiComponent(t *testing.T) {
	k, err := model.LoadKnowledge("../../knowledge/kserve.yaml")
	require.NoError(t, err)

	output, err := renderFuzzTargets(k)
	require.NoError(t, err)

	// Must parse as valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "fuzz_test.go", output, parser.AllErrors)
	require.NoError(t, err, "generated output is not valid Go:\n%s", output)

	// Must contain 4 Fuzz functions
	assert.Equal(t, 4, strings.Count(output, "func Fuzz"), "expected 4 Fuzz functions for kserve")
	assert.Contains(t, output, "FuzzKserveControllerManager")
	assert.Contains(t, output, "FuzzLlmisvcControllerManager")
	assert.Contains(t, output, "FuzzKserveLocalmodelControllerManager")
	assert.Contains(t, output, "FuzzKserveLocalmodelnodeAgent")
}
