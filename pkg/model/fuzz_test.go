package model

import (
	"testing"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk/fuzz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func TestSeedObjectsSingleComponent(t *testing.T) {
	k, err := LoadKnowledge("../../knowledge/dashboard.yaml")
	require.NoError(t, err)

	objects := SeedObjects(k)

	// dashboard.yaml has 7 managed resources: 1 Deployment, 1 ServiceAccount,
	// 3 ClusterRoleBindings, 1 Service, 1 ConfigMap
	assert.Len(t, objects, 7)

	// Check the Deployment has correct type, name, namespace, labels, replicas.
	var deploy *appsv1.Deployment
	for _, obj := range objects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			deploy = d
			break
		}
	}
	require.NotNil(t, deploy, "expected a typed *appsv1.Deployment")
	assert.Equal(t, "odh-dashboard", deploy.Name)
	assert.Equal(t, "opendatahub", deploy.Namespace)
	assert.Equal(t, "odh-dashboard", deploy.Labels["deployment"])
	assert.Equal(t, "odh-dashboard", deploy.Labels["app"])
	require.NotNil(t, deploy.Spec.Replicas)
	assert.Equal(t, int32(2), *deploy.Spec.Replicas)
	// Labels propagated to selector and template
	assert.Equal(t, "odh-dashboard", deploy.Spec.Selector.MatchLabels["deployment"])
	assert.Equal(t, "odh-dashboard", deploy.Spec.Template.Labels["deployment"])

	// Check typed ServiceAccount exists
	var foundSA bool
	for _, obj := range objects {
		if sa, ok := obj.(*corev1.ServiceAccount); ok && sa.Name == "odh-dashboard" {
			foundSA = true
			assert.Equal(t, "opendatahub", sa.Namespace)
		}
	}
	assert.True(t, foundSA, "expected ServiceAccount odh-dashboard")

	// Check typed ClusterRoleBindings exist (cluster-scoped, no namespace)
	var crbCount int
	for _, obj := range objects {
		if _, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
			crbCount++
		}
	}
	assert.Equal(t, 3, crbCount, "expected 3 ClusterRoleBindings")

	// Check typed Service exists
	var foundSvc bool
	for _, obj := range objects {
		if svc, ok := obj.(*corev1.Service); ok && svc.Name == "odh-dashboard" {
			foundSvc = true
		}
	}
	assert.True(t, foundSvc, "expected Service odh-dashboard")

	// Check typed ConfigMap exists
	var foundCM bool
	for _, obj := range objects {
		if cm, ok := obj.(*corev1.ConfigMap); ok && cm.Name == "kube-rbac-proxy-config" {
			foundCM = true
		}
	}
	assert.True(t, foundCM, "expected ConfigMap kube-rbac-proxy-config")
}

func TestSeedObjectsMultiComponent(t *testing.T) {
	k, err := LoadKnowledge("../../knowledge/kserve.yaml")
	require.NoError(t, err)

	objects := SeedObjects(k)

	// kserve.yaml has 4 components. Count total unique managed resources:
	// Component 1 (kserve-controller-manager): Deployment, ConfigMap, ServiceAccount, Secret, Lease = 5
	// Component 2 (llmisvc-controller-manager): Deployment, ServiceAccount, Secret, Lease = 4
	// Component 3 (kserve-localmodel-controller-manager): Deployment, ServiceAccount, Secret, Lease = 4
	// Component 4 (kserve-localmodelnode-agent): DaemonSet, ServiceAccount, Lease = 3
	// Total: 16 (all unique names)
	assert.Len(t, objects, 16)

	// Verify all components contribute objects
	names := make(map[string]bool)
	for _, obj := range objects {
		names[obj.GetName()] = true
	}
	assert.True(t, names["kserve-controller-manager"], "missing kserve-controller-manager")
	assert.True(t, names["llmisvc-controller-manager"], "missing llmisvc-controller-manager")
	assert.True(t, names["kserve-localmodel-controller-manager"], "missing kserve-localmodel-controller-manager")
	assert.True(t, names["kserve-localmodelnode-agent"], "missing kserve-localmodelnode-agent")

	// Verify DaemonSet is typed correctly
	var foundDS bool
	for _, obj := range objects {
		if _, ok := obj.(*appsv1.DaemonSet); ok {
			foundDS = true
		}
	}
	assert.True(t, foundDS, "expected a typed *appsv1.DaemonSet")
}

func TestSeedObjectsWebhooksAndFinalizers(t *testing.T) {
	k, err := LoadKnowledge("../../knowledge/model-registry.yaml")
	require.NoError(t, err)

	objects := SeedObjects(k)

	// model-registry: Deployment, ServiceAccount, ClusterRoleBinding, Lease, Service = 5
	assert.Len(t, objects, 5)

	// Verify Deployment has replicas
	var deploy *appsv1.Deployment
	for _, obj := range objects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			deploy = d
		}
	}
	require.NotNil(t, deploy)
	require.NotNil(t, deploy.Spec.Replicas)
	assert.Equal(t, int32(1), *deploy.Spec.Replicas)
}

func TestInvariantsSingleComponent(t *testing.T) {
	k, err := LoadKnowledge("../../knowledge/dashboard.yaml")
	require.NoError(t, err)

	invariants := Invariants(k)

	// dashboard.yaml has 1 conditionTrue check targeting Deployment odh-dashboard,
	// and 1 Deployment with replicas (same resource). Dedup means only 1 invariant.
	assert.Len(t, invariants, 1)

	// Verify the invariants are callable fuzz.Invariant functions
	var _ fuzz.Invariant = invariants[0]
}

func TestInvariantsMultiComponent(t *testing.T) {
	k, err := LoadKnowledge("../../knowledge/kserve.yaml")
	require.NoError(t, err)

	invariants := Invariants(k)

	// kserve.yaml has 4 components, each with 1 steady-state check = 4
	// 3 Deployments with replicas (components 1-3) = 3
	// But the steady-state checks target the same Deployments, so dedup
	// kicks in: the Deployment invariant for each component is already
	// covered by the steady-state check.
	// Component 4 has a DaemonSet check (resourceExists), no Deployment replicas.
	// Total: 4 (one per steady-state check, Deployment replicas all deduped)
	assert.Len(t, invariants, 4)
}

func TestSeedCorpusEntriesBase(t *testing.T) {
	k, err := LoadKnowledge("../../knowledge/dashboard.yaml")
	require.NoError(t, err)

	entries := SeedCorpusEntries(k)

	// dashboard.yaml: no webhooks, no finalizers, no Lease, no dependencies
	// Only base entry expected.
	assert.Len(t, entries, 1)
	assert.Equal(t, "base-api-errors", entries[0].Label)
	// OpMask should have Get (bit 0) and List (bit 1) set = 0x0003
	assert.Equal(t, uint16(0x0003), entries[0].OpMask)
}

func TestSeedCorpusEntriesWebhooks(t *testing.T) {
	k, err := LoadKnowledge("../../knowledge/model-registry.yaml")
	require.NoError(t, err)

	entries := SeedCorpusEntries(k)

	// model-registry: has webhooks + finalizers + Lease
	// base + webhook + finalizer + leader-election = 4
	assert.Len(t, entries, 4)

	labels := make(map[string]bool)
	for _, e := range entries {
		labels[e.Label] = true
	}
	assert.True(t, labels["base-api-errors"])
	assert.True(t, labels["webhook-rejection"])
	assert.True(t, labels["finalizer-conflict"])
	assert.True(t, labels["leader-election-contention"])
}

func TestSeedCorpusEntriesLeaderElection(t *testing.T) {
	k, err := LoadKnowledge("../../knowledge/kserve.yaml")
	require.NoError(t, err)

	entries := SeedCorpusEntries(k)

	// kserve: has webhooks + finalizers + Lease + dependencies
	// base + webhook + finalizer + leader-election + dependency = 5
	assert.Len(t, entries, 5)

	labels := make(map[string]bool)
	for _, e := range entries {
		labels[e.Label] = true
	}
	assert.True(t, labels["leader-election-contention"])
}

func TestSeedCorpusEntriesDependencies(t *testing.T) {
	k, err := LoadKnowledge("../../knowledge/llamastack.yaml")
	require.NoError(t, err)

	entries := SeedCorpusEntries(k)

	// llamastack: has webhooks + finalizers + Lease + dependencies
	// base + webhook + finalizer + leader-election + dependency = 5
	assert.Len(t, entries, 5)

	labels := make(map[string]bool)
	for _, e := range entries {
		labels[e.Label] = true
	}
	assert.True(t, labels["dependency-unavailable"])
}
