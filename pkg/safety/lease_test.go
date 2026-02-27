package safety

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLeaseExperimentLockAcquire(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	lock := NewLeaseExperimentLock(client, "opendatahub")
	err := lock.Acquire(context.Background(), "test-operator", "test-experiment")
	assert.NoError(t, err)
}

func TestLeaseExperimentLockConflict(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	lock := NewLeaseExperimentLock(client, "opendatahub")
	err := lock.Acquire(context.Background(), "test-operator", "experiment-1")
	require.NoError(t, err)

	err = lock.Acquire(context.Background(), "test-operator", "experiment-2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "experiment-1")
}

func TestLeaseExperimentLockRelease(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	lock := NewLeaseExperimentLock(client, "opendatahub")
	err := lock.Acquire(context.Background(), "test-operator", "experiment-1")
	require.NoError(t, err)

	lock.Release("test-operator")

	// Should be able to acquire again after release
	err = lock.Acquire(context.Background(), "test-operator", "experiment-2")
	assert.NoError(t, err)
}

func TestLeaseExperimentLockDifferentOperators(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	lock := NewLeaseExperimentLock(client, "opendatahub")

	err := lock.Acquire(context.Background(), "operator-a", "exp-1")
	require.NoError(t, err)

	// Different operator should work
	err = lock.Acquire(context.Background(), "operator-b", "exp-2")
	assert.NoError(t, err)

	lock.Release("operator-a")
	lock.Release("operator-b")
}

func TestLeaseExperimentLockSetsExpiry(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	lock := NewLeaseExperimentLock(c, "opendatahub")
	err := lock.Acquire(context.Background(), "test-operator", "test-experiment")
	require.NoError(t, err)

	// Fetch the created lease and verify expiry fields are set.
	lease := &coordinationv1.Lease{}
	err = c.Get(context.Background(), client.ObjectKey{
		Name:      "odh-chaos-lock-test-operator",
		Namespace: "opendatahub",
	}, lease)
	require.NoError(t, err)

	assert.NotNil(t, lease.Spec.LeaseDurationSeconds, "LeaseDurationSeconds should be set")
	assert.Equal(t, DefaultLeaseDurationSeconds, *lease.Spec.LeaseDurationSeconds)
	assert.NotNil(t, lease.Spec.AcquireTime, "AcquireTime should be set")
	assert.WithinDuration(t, time.Now(), lease.Spec.AcquireTime.Time, 5*time.Second)
}

func TestLeaseExperimentLockExpiry(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	// Create a pre-expired lease: acquired 1 hour ago with a 15-minute duration.
	expiredHolder := "crashed-experiment"
	expiredDuration := DefaultLeaseDurationSeconds
	expiredTime := metav1.NewMicroTime(time.Now().Add(-1 * time.Hour))
	expiredLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-chaos-lock-test-operator",
			Namespace: "opendatahub",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "odh-chaos",
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &expiredHolder,
			LeaseDurationSeconds: &expiredDuration,
			AcquireTime:          &expiredTime,
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(expiredLease).Build()
	lock := NewLeaseExperimentLock(c, "opendatahub")

	// Acquire should succeed by reclaiming (updating) the stale lease in-place.
	err := lock.Acquire(context.Background(), "test-operator", "new-experiment")
	assert.NoError(t, err)

	// Verify the new lease has the correct holder.
	lease := &coordinationv1.Lease{}
	err = c.Get(context.Background(), client.ObjectKey{
		Name:      "odh-chaos-lock-test-operator",
		Namespace: "opendatahub",
	}, lease)
	require.NoError(t, err)
	assert.Equal(t, "new-experiment", *lease.Spec.HolderIdentity)
}

func TestLeaseExperimentLockAcquireUsesUpdateForExpiredLease(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	// Create a pre-expired lease: acquired 2 hours ago with a 15-minute duration.
	expiredHolder := "old-experiment"
	expiredDuration := DefaultLeaseDurationSeconds
	expiredTime := metav1.NewMicroTime(time.Now().Add(-2 * time.Hour))
	expiredLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-chaos-lock-my-operator",
			Namespace: "opendatahub",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "odh-chaos",
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &expiredHolder,
			LeaseDurationSeconds: &expiredDuration,
			AcquireTime:          &expiredTime,
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(expiredLease).Build()
	lock := NewLeaseExperimentLock(c, "opendatahub")

	// Acquire should reclaim the expired lease by updating it in-place
	// (not deleting + recreating), which avoids TOCTOU race conditions.
	err := lock.Acquire(context.Background(), "my-operator", "reclaimer-experiment")
	require.NoError(t, err)

	// Verify the lease was updated (not deleted and recreated) by checking:
	// 1. The holder changed to the new experiment
	lease := &coordinationv1.Lease{}
	err = c.Get(context.Background(), client.ObjectKey{
		Name:      "odh-chaos-lock-my-operator",
		Namespace: "opendatahub",
	}, lease)
	require.NoError(t, err)
	assert.Equal(t, "reclaimer-experiment", *lease.Spec.HolderIdentity,
		"holder should be updated to new experiment")

	// 2. AcquireTime should be recent (within last 5 seconds)
	assert.WithinDuration(t, time.Now(), lease.Spec.AcquireTime.Time, 5*time.Second,
		"acquire time should be updated to now")

	// 3. LeaseDurationSeconds should still be the default
	assert.Equal(t, DefaultLeaseDurationSeconds, *lease.Spec.LeaseDurationSeconds,
		"lease duration should be set to default")

	// 4. The managed-by label should still be present (preserved from the original object)
	assert.Equal(t, "odh-chaos", lease.Labels["app.kubernetes.io/managed-by"],
		"managed-by label should be preserved")
}
