package orchestrator

import (
	"bytes"
	"context"
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/evaluator"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/injection"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock observer
type mockObserver struct {
	result *v1alpha1.CheckResult
}

func (m *mockObserver) CheckSteadyState(ctx context.Context, checks []v1alpha1.SteadyStateCheck, namespace string) *v1alpha1.CheckResult {
	if m.result != nil {
		return m.result
	}
	return &v1alpha1.CheckResult{Passed: true, ChecksRun: 0, Timestamp: time.Now()}
}

// Mock injector
type mockInjector struct {
	validateErr   error
	injectErr     error
	cleanupCalled bool
}

func (m *mockInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return m.validateErr
}

func (m *mockInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (injection.CleanupFunc, []v1alpha1.InjectionEvent, error) {
	if m.injectErr != nil {
		return nil, nil, m.injectErr
	}
	events := []v1alpha1.InjectionEvent{
		{Type: spec.Type, Target: "test-pod", Action: "deleted", Timestamp: time.Now()},
	}
	cleanup := func(ctx context.Context) error {
		m.cleanupCalled = true
		return nil
	}
	return cleanup, events, nil
}

func newTestOrchestrator(obs *mockObserver, inj *mockInjector) *Orchestrator {
	registry := injection.NewRegistry()
	registry.Register(v1alpha1.PodKill, inj)

	orch := New(OrchestratorConfig{
		Registry:  registry,
		Observer:  obs,
		Evaluator: evaluator.New(10),
		Lock:      safety.NewLocalExperimentLock(),
		Verbose:   false,
	})

	// Redirect output to a buffer for tests
	orch.output = &bytes.Buffer{}

	return orch
}

func newTestExperiment() *v1alpha1.ChaosExperiment {
	return &v1alpha1.ChaosExperiment{
		Metadata: v1alpha1.Metadata{
			Name:      "test-experiment",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.ChaosExperimentSpec{
			Target: v1alpha1.TargetSpec{
				Operator:  "test-operator",
				Component: "dashboard",
			},
			Injection: v1alpha1.InjectionSpec{
				Type:  v1alpha1.PodKill,
				Count: 1,
				Parameters: map[string]string{
					"labelSelector": "app=dashboard",
				},
			},
			BlastRadius: v1alpha1.BlastRadiusSpec{
				MaxPodsAffected:   1,
				AllowedNamespaces: []string{"test-ns"},
			},
			Hypothesis: v1alpha1.HypothesisSpec{
				Description:     "Test recovers",
				RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second},
			},
		},
	}
}

func TestOrchestratorHappyPath(t *testing.T) {
	obs := &mockObserver{result: &v1alpha1.CheckResult{Passed: true, ChecksRun: 1, ChecksPassed: 1, Timestamp: time.Now()}}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	result, err := orch.Run(context.Background(), newTestExperiment())
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.PhaseComplete, result.Phase)
	assert.Equal(t, v1alpha1.Resilient, result.Verdict)
	assert.True(t, inj.cleanupCalled)
}

func TestOrchestratorDryRun(t *testing.T) {
	obs := &mockObserver{}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	exp := newTestExperiment()
	exp.Spec.BlastRadius.DryRun = true

	result, err := orch.Run(context.Background(), exp)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.PhaseComplete, result.Phase)
	assert.False(t, inj.cleanupCalled) // Should not inject in dry run
}

func TestOrchestratorBlastRadiusViolation(t *testing.T) {
	obs := &mockObserver{}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	exp := newTestExperiment()
	exp.Spec.BlastRadius.MaxPodsAffected = 0 // Invalid

	result, err := orch.Run(context.Background(), exp)
	assert.Error(t, err)
	assert.Equal(t, v1alpha1.PhaseAborted, result.Phase)
}

func TestOrchestratorPreCheckFailed(t *testing.T) {
	obs := &mockObserver{result: &v1alpha1.CheckResult{Passed: false, ChecksRun: 1, ChecksPassed: 0, Timestamp: time.Now()}}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	exp := newTestExperiment()
	exp.Spec.SteadyState = v1alpha1.SteadyStateDef{
		Checks: []v1alpha1.SteadyStateCheck{{Type: "conditionTrue"}},
	}

	result, err := orch.Run(context.Background(), exp)
	assert.Error(t, err)
	assert.Equal(t, v1alpha1.PhaseAborted, result.Phase)
	assert.Equal(t, v1alpha1.Inconclusive, result.Verdict)
}

func TestOrchestratorCleanupOnError(t *testing.T) {
	// Verify cleanup is called even when later phases fail
	obs := &mockObserver{}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	result, err := orch.Run(context.Background(), newTestExperiment())
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.PhaseComplete, result.Phase)
	assert.True(t, inj.cleanupCalled)
}

func TestOrchestratorInjectionError(t *testing.T) {
	obs := &mockObserver{result: &v1alpha1.CheckResult{Passed: true, ChecksRun: 1, ChecksPassed: 1, Timestamp: time.Now()}}
	inj := &mockInjector{injectErr: assert.AnError}
	orch := newTestOrchestrator(obs, inj)

	exp := newTestExperiment()
	exp.Spec.SteadyState = v1alpha1.SteadyStateDef{
		Checks: []v1alpha1.SteadyStateCheck{{Type: "conditionTrue"}},
	}

	result, err := orch.Run(context.Background(), exp)
	assert.Error(t, err)
	assert.Equal(t, v1alpha1.PhaseAborted, result.Phase)
	assert.Contains(t, result.Error, "injection failed")
}

func TestOrchestratorValidationError(t *testing.T) {
	obs := &mockObserver{}
	inj := &mockInjector{validateErr: assert.AnError}
	orch := newTestOrchestrator(obs, inj)

	result, err := orch.Run(context.Background(), newTestExperiment())
	assert.Error(t, err)
	assert.Equal(t, v1alpha1.PhaseAborted, result.Phase)
	assert.Contains(t, result.Error, "injection validation failed")
}

func TestOrchestratorDangerLevelBlocked(t *testing.T) {
	obs := &mockObserver{}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	exp := newTestExperiment()
	exp.Spec.Injection.DangerLevel = "high"
	exp.Spec.BlastRadius.AllowDangerous = false

	result, err := orch.Run(context.Background(), exp)
	assert.Error(t, err)
	assert.Equal(t, v1alpha1.PhaseAborted, result.Phase)
	assert.Contains(t, result.Error, "danger level")
}

func TestOrchestratorDefaultNamespace(t *testing.T) {
	obs := &mockObserver{result: &v1alpha1.CheckResult{Passed: true, ChecksRun: 0, Timestamp: time.Now()}}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	exp := newTestExperiment()
	exp.Metadata.Namespace = "" // empty namespace should default to "opendatahub"
	exp.Spec.BlastRadius.AllowedNamespaces = []string{"opendatahub"}

	result, err := orch.Run(context.Background(), exp)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.PhaseComplete, result.Phase)
}

func TestOrchestratorUnknownInjectionType(t *testing.T) {
	obs := &mockObserver{}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	exp := newTestExperiment()
	exp.Spec.Injection.Type = "UnknownType"

	result, err := orch.Run(context.Background(), exp)
	assert.Error(t, err)
	assert.Equal(t, v1alpha1.PhaseAborted, result.Phase)
	assert.Contains(t, result.Error, "unknown injection type")
}

func TestOrchestratorLogOutput(t *testing.T) {
	obs := &mockObserver{result: &v1alpha1.CheckResult{Passed: true, ChecksRun: 1, ChecksPassed: 1, Timestamp: time.Now()}}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	buf := &bytes.Buffer{}
	orch.output = buf

	_, err := orch.Run(context.Background(), newTestExperiment())
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "PENDING")
	assert.Contains(t, output, "COMPLETE")
}

func TestOrchestratorReportGeneration(t *testing.T) {
	obs := &mockObserver{result: &v1alpha1.CheckResult{Passed: true, ChecksRun: 1, ChecksPassed: 1, Timestamp: time.Now()}}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)

	result, err := orch.Run(context.Background(), newTestExperiment())
	require.NoError(t, err)
	require.NotNil(t, result.Report)
	assert.Equal(t, "test-experiment", result.Report.Experiment)
	assert.Equal(t, "test-operator", result.Report.Target.Operator)
	assert.Equal(t, "dashboard", result.Report.Target.Component)
}

func TestOrchestratorReportWrittenToDir(t *testing.T) {
	obs := &mockObserver{result: &v1alpha1.CheckResult{Passed: true, ChecksRun: 1, ChecksPassed: 1, Timestamp: time.Now()}}
	inj := &mockInjector{}
	orch := newTestOrchestrator(obs, inj)
	orch.reportDir = t.TempDir()

	result, err := orch.Run(context.Background(), newTestExperiment())
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.PhaseComplete, result.Phase)
	// Report should have been written to the temp dir
	require.NotNil(t, result.Report)
}
