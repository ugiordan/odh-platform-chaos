package reporter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJUnitReporter(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnitReporter(&buf)

	reports := []ExperimentReport{
		{
			Experiment: "test-resilient",
			Timestamp:  time.Now(),
			Evaluation: evaluator.EvaluationResult{
				Verdict:      v1alpha1.Resilient,
				RecoveryTime: 12 * time.Second,
			},
		},
		{
			Experiment: "test-failed",
			Timestamp:  time.Now(),
			Evaluation: evaluator.EvaluationResult{
				Verdict: v1alpha1.Failed,
			},
		},
	}

	err := r.WriteSuite("chaos-tests", reports)
	require.NoError(t, err)

	output := buf.String()
	assert.True(t, strings.Contains(output, "<testsuite"))
	assert.True(t, strings.Contains(output, "test-resilient"))
	assert.True(t, strings.Contains(output, "test-failed"))
	assert.True(t, strings.Contains(output, "<failure"))
}
