package reporter

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONReporterWrite(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)

	report := ExperimentReport{
		Experiment: "dashboard-pod-kill",
		Timestamp:  time.Date(2026, 2, 26, 10, 0, 0, 0, time.UTC),
		Target: TargetReport{
			Operator:  "opendatahub-operator",
			Component: "dashboard",
			Resource:  "Deployment/odh-dashboard",
		},
		Injection: InjectionReport{
			Type:      string(v1alpha1.PodKill),
			Timestamp: time.Date(2026, 2, 26, 10, 0, 5, 0, time.UTC),
		},
		Evaluation: evaluator.EvaluationResult{
			Verdict:      v1alpha1.Resilient,
			Confidence:   "3/3 checks passed, 12s recovery, 2 cycles",
			RecoveryTime: 12 * time.Second,
		},
	}

	err := r.Write(report)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "dashboard-pod-kill", parsed["experiment"])
	assert.Equal(t, "Resilient", parsed["evaluation"].(map[string]interface{})["verdict"])
}

func TestJSONReporterToFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/report.json"

	r, err := NewJSONFileReporter(path)
	require.NoError(t, err)

	report := ExperimentReport{
		Experiment: "test",
		Timestamp:  time.Now(),
		Evaluation: evaluator.EvaluationResult{Verdict: v1alpha1.Failed},
	}

	err = r.Write(report)
	require.NoError(t, err)
}
