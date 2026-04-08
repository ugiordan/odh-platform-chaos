package model

import (
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"
	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk/fuzz"
)

// SeedObjects creates minimal K8s objects from an OperatorKnowledge model.
// Each ManagedResource is mapped to its correct typed Go object with GVK,
// name, namespace, and labels populated. Duplicates (same kind+name+namespace)
// across components are emitted only once.
func SeedObjects(k *OperatorKnowledge) []client.Object {
	seen := make(map[string]bool)
	var objects []client.Object

	for _, comp := range k.Components {
		for _, mr := range comp.ManagedResources {
			key := mr.Kind + "/" + mr.Namespace + "/" + mr.Name
			if seen[key] {
				continue
			}
			seen[key] = true

			obj := resourceToObject(mr)
			objects = append(objects, obj)
		}
	}

	return objects
}

func resourceToObject(mr ManagedResource) client.Object {
	switch mr.Kind {
	case "Deployment":
		d := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mr.Name,
				Namespace: mr.Namespace,
				Labels:    mr.Labels,
			},
		}
		if r, ok := mr.ExpectedSpec["replicas"]; ok {
			if rf, ok := r.(float64); ok {
				d.Spec.Replicas = ptr.To(int32(rf))
			} else if ri, ok := r.(int); ok {
				d.Spec.Replicas = ptr.To(int32(ri))
			}
		}
		if len(mr.Labels) > 0 {
			d.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: copyLabels(mr.Labels),
			}
			d.Spec.Template.Labels = copyLabels(mr.Labels)
		}
		return d

	case "DaemonSet":
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mr.Name,
				Namespace: mr.Namespace,
				Labels:    mr.Labels,
			},
		}
		if len(mr.Labels) > 0 {
			ds.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: copyLabels(mr.Labels),
			}
			ds.Spec.Template.Labels = copyLabels(mr.Labels)
		}
		return ds

	case "ServiceAccount":
		return &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mr.Name,
				Namespace: mr.Namespace,
			},
		}

	case "Service":
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mr.Name,
				Namespace: mr.Namespace,
			},
		}

	case "ConfigMap":
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mr.Name,
				Namespace: mr.Namespace,
			},
		}

	case "Secret":
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mr.Name,
				Namespace: mr.Namespace,
			},
		}

	case "Lease":
		return &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mr.Name,
				Namespace: mr.Namespace,
			},
		}

	case "ClusterRoleBinding":
		return &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: mr.Name,
			},
		}

	case "ClusterRole":
		return &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: mr.Name,
			},
		}

	default:
		u := &unstructured.Unstructured{}
		u.SetAPIVersion(mr.APIVersion)
		u.SetKind(mr.Kind)
		u.SetName(mr.Name)
		u.SetNamespace(mr.Namespace)
		return u
	}
}

func copyLabels(labels map[string]string) map[string]string {
	cp := make(map[string]string, len(labels))
	for k, v := range labels {
		cp[k] = v
	}
	return cp
}

// Invariants creates fuzz invariant functions from an OperatorKnowledge model.
// For each steady-state check, it creates an ObjectExists invariant that verifies
// the target resource survives reconciliation. For each Deployment with expected
// replicas, it adds an ObjectExists invariant.
//
// Objects passed to invariant closures are DeepCopy'd from fresh instances
// to prevent shared mutable state with seed objects.
func Invariants(k *OperatorKnowledge) []fuzz.Invariant {
	seen := make(map[string]bool)
	var invariants []fuzz.Invariant

	for _, comp := range k.Components {
		// Invariants from steady-state checks
		for _, check := range comp.SteadyState.Checks {
			key := check.Kind + "/" + check.Namespace + "/" + check.Name
			if seen[key] {
				continue
			}
			seen[key] = true

			obj := checkToObject(check)
			invariants = append(invariants, fuzz.ObjectExists(
				types.NamespacedName{Name: check.Name, Namespace: check.Namespace},
				obj,
			))
		}

		// Invariants from Deployments with replicas
		for _, mr := range comp.ManagedResources {
			if mr.Kind != "Deployment" {
				continue
			}
			if _, hasReplicas := mr.ExpectedSpec["replicas"]; !hasReplicas {
				continue
			}
			key := "Deployment/" + mr.Namespace + "/" + mr.Name
			if seen[key] {
				continue
			}
			seen[key] = true

			obj := &appsv1.Deployment{}
			invariants = append(invariants, fuzz.ObjectExists(
				types.NamespacedName{Name: mr.Name, Namespace: mr.Namespace},
				obj.DeepCopy(),
			))
		}
	}

	return invariants
}

// checkToObject creates a fresh typed object for a SteadyStateCheck.
// The object is DeepCopy'd to prevent shared mutable state.
func checkToObject(check v1alpha1.SteadyStateCheck) client.Object {
	switch check.Kind {
	case "Deployment":
		return (&appsv1.Deployment{}).DeepCopy()
	case "DaemonSet":
		return (&appsv1.DaemonSet{}).DeepCopy()
	case "ConfigMap":
		return (&corev1.ConfigMap{}).DeepCopy()
	case "Service":
		return (&corev1.Service{}).DeepCopy()
	case "ServiceAccount":
		return (&corev1.ServiceAccount{}).DeepCopy()
	case "Secret":
		return (&corev1.Secret{}).DeepCopy()
	default:
		u := &unstructured.Unstructured{}
		u.SetAPIVersion(check.APIVersion)
		u.SetKind(check.Kind)
		return u
	}
}

// SeedCorpusEntry represents an encoded fuzz seed corpus value.
// These map to f.Add(e.OpMask, e.FaultType, e.Intensity) calls
// in generated fuzz tests, giving the fuzzer architecturally-relevant
// starting points.
type SeedCorpusEntry struct {
	Label     string // human-readable description
	OpMask    uint16
	FaultType uint8
	Intensity uint16
}

// opBit returns the bit position for an sdk.Operation in the fuzz opMask.
var opBit = map[sdk.Operation]uint{
	sdk.OpGet:         0,
	sdk.OpList:        1,
	sdk.OpCreate:      2,
	sdk.OpUpdate:      3,
	sdk.OpDelete:      4,
	sdk.OpPatch:       5,
	sdk.OpDeleteAllOf: 6,
	sdk.OpReconcile:   7,
	sdk.OpApply:       8,
}

// faultTypeIndex maps error message categories to faultType byte values.
// These must match the faultMessages table in pkg/sdk/fuzz/decode.go.
var faultTypeIndex = map[string]uint8{
	"conflict":     0,
	"not-found":    1,
	"timeout":      2,
	"server-error": 3,
	"etcd":         4,
	"throttle":     5,
	"connection":   6,
	"gone":         7,
	"webhook":      8,
	"quota":        9,
	"unavailable":  10,
}

// intensityFromRate converts a float64 error rate (0.0-1.0) to uint16 (0-65535).
func intensityFromRate(rate float64) uint16 {
	return uint16(rate * 65535)
}

// opMask builds a bitmask from a set of operations.
func opMask(ops ...sdk.Operation) uint16 {
	var mask uint16
	for _, op := range ops {
		mask |= 1 << opBit[op]
	}
	return mask
}

// SeedCorpusEntries maps architectural traits from an OperatorKnowledge model
// to encoded fuzz seed corpus entries. Each entry encodes a (opMask, faultType,
// intensity) tuple that DecodeFaultConfig can reconstruct into a FaultConfig.
func SeedCorpusEntries(k *OperatorKnowledge) []SeedCorpusEntry {
	var entries []SeedCorpusEntry

	// Base: every operator gets Get+List with connection errors
	entries = append(entries, SeedCorpusEntry{
		Label:     "base-api-errors",
		OpMask:    opMask(sdk.OpGet, sdk.OpList),
		FaultType: faultTypeIndex["connection"],
		Intensity: intensityFromRate(0.3),
	})

	hasWebhooks := false
	hasFinalizers := false
	hasLease := false
	hasDependencies := false

	for _, comp := range k.Components {
		if len(comp.Webhooks) > 0 {
			hasWebhooks = true
		}
		if len(comp.Finalizers) > 0 {
			hasFinalizers = true
		}
		if len(comp.Dependencies) > 0 {
			hasDependencies = true
		}
		for _, mr := range comp.ManagedResources {
			if mr.Kind == "Lease" {
				hasLease = true
			}
		}
	}

	if hasWebhooks {
		entries = append(entries, SeedCorpusEntry{
			Label:     "webhook-rejection",
			OpMask:    opMask(sdk.OpCreate, sdk.OpUpdate),
			FaultType: faultTypeIndex["webhook"],
			Intensity: intensityFromRate(0.5),
		})
	}

	if hasFinalizers {
		entries = append(entries, SeedCorpusEntry{
			Label:     "finalizer-conflict",
			OpMask:    opMask(sdk.OpDelete),
			FaultType: faultTypeIndex["conflict"],
			Intensity: intensityFromRate(0.5),
		})
	}

	if hasLease {
		entries = append(entries, SeedCorpusEntry{
			Label:     "leader-election-contention",
			OpMask:    opMask(sdk.OpGet, sdk.OpUpdate),
			FaultType: faultTypeIndex["timeout"],
			Intensity: intensityFromRate(0.4),
		})
	}

	if hasDependencies {
		entries = append(entries, SeedCorpusEntry{
			Label:     "dependency-unavailable",
			OpMask:    opMask(sdk.OpGet, sdk.OpList),
			FaultType: faultTypeIndex["not-found"],
			Intensity: intensityFromRate(0.6),
		})
	}

	return entries
}

// SeedObjectCode returns a Go source code string that constructs a seed object
// for the given ManagedResource. Used by the code generator.
func SeedObjectCode(mr ManagedResource) string {
	switch mr.Kind {
	case "Deployment":
		code := fmt.Sprintf(`&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: %q,
				Namespace: %q,`, mr.Name, mr.Namespace)
		if len(mr.Labels) > 0 {
			code += fmt.Sprintf(`
				Labels: %s,`, labelsLiteral(mr.Labels))
		}
		code += `
			},`
		if _, ok := mr.ExpectedSpec["replicas"]; ok || len(mr.Labels) > 0 {
			code += `
			Spec: appsv1.DeploymentSpec{`
			if r, ok := mr.ExpectedSpec["replicas"]; ok {
				var replicas int
				switch v := r.(type) {
				case float64:
					replicas = int(v)
				case int:
					replicas = v
				}
				code += fmt.Sprintf(`
				Replicas: ptr.To[int32](%d),`, replicas)
			}
			if len(mr.Labels) > 0 {
				code += fmt.Sprintf(`
				Selector: &metav1.LabelSelector{
					MatchLabels: %s,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: %s,
					},
				},`, labelsLiteral(mr.Labels), labelsLiteral(mr.Labels))
			}
			code += `
			},`
		}
		code += `
		}`
		return code

	case "DaemonSet":
		code := fmt.Sprintf(`&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: %q,
				Namespace: %q,`, mr.Name, mr.Namespace)
		if len(mr.Labels) > 0 {
			code += fmt.Sprintf(`
				Labels: %s,`, labelsLiteral(mr.Labels))
		}
		code += `
			},`
		if len(mr.Labels) > 0 {
			code += fmt.Sprintf(`
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: %s,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: %s,
					},
				},
			},`, labelsLiteral(mr.Labels), labelsLiteral(mr.Labels))
		}
		code += `
		}`
		return code

	case "ServiceAccount":
		return fmt.Sprintf(`&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: %q,
				Namespace: %q,
			},
		}`, mr.Name, mr.Namespace)

	case "Service":
		return fmt.Sprintf(`&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: %q,
				Namespace: %q,
			},
		}`, mr.Name, mr.Namespace)

	case "ConfigMap":
		return fmt.Sprintf(`&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: %q,
				Namespace: %q,
			},
		}`, mr.Name, mr.Namespace)

	case "Secret":
		return fmt.Sprintf(`&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: %q,
				Namespace: %q,
			},
		}`, mr.Name, mr.Namespace)

	case "Lease":
		return fmt.Sprintf(`&coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name: %q,
				Namespace: %q,
			},
		}`, mr.Name, mr.Namespace)

	case "ClusterRoleBinding":
		return fmt.Sprintf(`&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: %q,
			},
		}`, mr.Name)

	case "ClusterRole":
		return fmt.Sprintf(`&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: %q,
			},
		}`, mr.Name)

	default:
		return fmt.Sprintf(`// unsupported kind %q: %s/%s`, mr.Kind, mr.Namespace, mr.Name)
	}
}

// InvariantCode returns Go source code for adding an invariant to a harness.
func InvariantCode(kind, name, namespace string) string {
	var typeLiteral string
	switch kind {
	case "Deployment":
		typeLiteral = "&appsv1.Deployment{}"
	case "DaemonSet":
		typeLiteral = "&appsv1.DaemonSet{}"
	case "ConfigMap":
		typeLiteral = "&corev1.ConfigMap{}"
	case "Service":
		typeLiteral = "&corev1.Service{}"
	case "ServiceAccount":
		typeLiteral = "&corev1.ServiceAccount{}"
	case "Secret":
		typeLiteral = "&corev1.Secret{}"
	default:
		typeLiteral = "&corev1.ConfigMap{}"
	}
	return fmt.Sprintf(
		`h.AddInvariant(fuzz.ObjectExists(types.NamespacedName{Name: %q, Namespace: %q}, %s))`,
		name, namespace, typeLiteral,
	)
}

func labelsLiteral(labels map[string]string) string {
	if len(labels) == 0 {
		return "nil"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%q: %q", k, labels[k]))
	}
	return "map[string]string{" + strings.Join(pairs, ", ") + "}"
}
