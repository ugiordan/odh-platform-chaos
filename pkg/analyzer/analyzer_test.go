package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeFile(t *testing.T) {
	findings, err := AnalyzeFile("../../testdata/go-source/sample_controller.go")
	require.NoError(t, err)
	assert.NotEmpty(t, findings)
}

func TestAnalyzeFileNotFound(t *testing.T) {
	_, err := AnalyzeFile("nonexistent.go")
	assert.Error(t, err)
}

func TestFindingsContainIgnoredErrors(t *testing.T) {
	findings, err := AnalyzeFile("../../testdata/go-source/sample_controller.go")
	require.NoError(t, err)

	hasIgnoredError := false
	for _, f := range findings {
		if f.Type == PatternIgnoredError {
			hasIgnoredError = true
			break
		}
	}
	assert.True(t, hasIgnoredError, "should detect ignored errors")
}

func TestFindingsContainGoroutines(t *testing.T) {
	findings, err := AnalyzeFile("../../testdata/go-source/sample_controller.go")
	require.NoError(t, err)

	hasGoroutine := false
	for _, f := range findings {
		if f.Type == PatternGoroutineLaunch {
			hasGoroutine = true
			break
		}
	}
	assert.True(t, hasGoroutine, "should detect goroutine launches")
}

func TestFindingsContainNetworkCalls(t *testing.T) {
	findings, err := AnalyzeFile("../../testdata/go-source/sample_controller.go")
	require.NoError(t, err)

	hasNetworkCall := false
	for _, f := range findings {
		if f.Type == PatternNetworkCall {
			hasNetworkCall = true
			break
		}
	}
	assert.True(t, hasNetworkCall, "should detect network calls")
}
