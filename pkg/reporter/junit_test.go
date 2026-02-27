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

func TestJUnitReporterSystemErr(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnitReporter(&buf)

	reports := []ExperimentReport{
		{
			Experiment: "test-with-cleanup-error",
			Timestamp:  time.Now(),
			Target: TargetReport{
				Component: "dashboard",
			},
			Evaluation: evaluator.EvaluationResult{
				Verdict:      v1alpha1.Resilient,
				RecoveryTime: 5 * time.Second,
			},
			CleanupError: "failed to restore pod",
		},
		{
			Experiment: "test-without-cleanup-error",
			Timestamp:  time.Now(),
			Target: TargetReport{
				Component: "controller",
			},
			Evaluation: evaluator.EvaluationResult{
				Verdict:      v1alpha1.Resilient,
				RecoveryTime: 3 * time.Second,
			},
		},
	}

	err := r.WriteSuite("cleanup-tests", reports)
	require.NoError(t, err)

	output := buf.String()
	assert.True(t, strings.Contains(output, "<system-err>failed to restore pod</system-err>"),
		"JUnit output should contain system-err element for cleanup errors")
	// The test case without cleanup error should not have system-err
	assert.Equal(t, 1, strings.Count(output, "<system-err>"),
		"Only one system-err element should be present")
}
