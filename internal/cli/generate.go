package cli

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"strings"
	"text/template"
	"unicode"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/model"
	"github.com/spf13/cobra"
)

func newGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate code from knowledge models",
	}

	cmd.AddCommand(newGenerateFuzzTargetsCommand())

	return cmd
}

func newGenerateFuzzTargetsCommand() *cobra.Command {
	var knowledgePath string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "fuzz-targets",
		Short: "Generate fuzz test targets from a knowledge model",
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := model.LoadKnowledge(knowledgePath)
			if err != nil {
				return fmt.Errorf("loading knowledge: %w", err)
			}

			errs := model.ValidateKnowledge(k)
			if len(errs) > 0 {
				return fmt.Errorf("knowledge validation failed: %v", errs[0])
			}

			output, err := renderFuzzTargets(k)
			if err != nil {
				return fmt.Errorf("rendering template: %w", err)
			}

			if outputPath == "" {
				fmt.Print(output)
				return nil
			}

			return os.WriteFile(outputPath, []byte(output), 0644)
		},
	}

	cmd.Flags().StringVar(&knowledgePath, "knowledge", "", "path to knowledge YAML file (required)")
	cmd.Flags().StringVar(&outputPath, "output", "", "output file path (defaults to stdout)")
	_ = cmd.MarkFlagRequired("knowledge")

	return cmd
}

type templateData struct {
	OperatorName string
	Components   []templateComponent
}

type templateComponent struct {
	FuncName    string
	Name        string
	SeedObjects string // Go code for seed object slice elements
	Request     string // Go code for reconcile.Request
	Invariants  string // Go code for invariant additions
	SeedCorpus  string // Go code for f.Add() calls
}

func renderFuzzTargets(k *model.OperatorKnowledge) (string, error) {
	data := templateData{
		OperatorName: k.Operator.Name,
	}

	corpusEntries := model.SeedCorpusEntries(k)

	for _, comp := range k.Components {
		tc := templateComponent{
			FuncName: "Fuzz" + pascalCase(comp.Name),
			Name:     comp.Name,
		}

		// Find request target (prefer Deployment, then DaemonSet, then first resource)
		for _, mr := range comp.ManagedResources {
			if mr.Kind == "Deployment" || mr.Kind == "DaemonSet" {
				tc.Request = fmt.Sprintf(
					`reconcile.Request{NamespacedName: types.NamespacedName{Name: %q, Namespace: %q}}`,
					mr.Name, mr.Namespace,
				)
				break
			}
		}
		if tc.Request == "" && len(comp.ManagedResources) > 0 {
			mr := comp.ManagedResources[0]
			tc.Request = fmt.Sprintf(
				`reconcile.Request{NamespacedName: types.NamespacedName{Name: %q, Namespace: %q}}`,
				mr.Name, mr.Namespace,
			)
		}

		// Seed objects: one per managed resource
		var seedLines []string
		for _, mr := range comp.ManagedResources {
			seedLines = append(seedLines, model.SeedObjectCode(mr)+",")
		}
		tc.SeedObjects = strings.Join(seedLines, "\n\t\t")

		// Invariants: from steady-state checks + Deployment replicas
		seen := make(map[string]bool)
		var invLines []string
		for _, check := range comp.SteadyState.Checks {
			key := check.Kind + "/" + check.Namespace + "/" + check.Name
			if !seen[key] {
				seen[key] = true
				invLines = append(invLines, model.InvariantCode(check.Kind, check.Name, check.Namespace))
			}
		}
		for _, mr := range comp.ManagedResources {
			if mr.Kind == "Deployment" {
				if _, ok := mr.ExpectedSpec["replicas"]; ok {
					key := "Deployment/" + mr.Namespace + "/" + mr.Name
					if !seen[key] {
						seen[key] = true
						invLines = append(invLines, model.InvariantCode("Deployment", mr.Name, mr.Namespace))
					}
				}
			}
		}
		tc.Invariants = strings.Join(invLines, "\n\t\t\t")

		// Seed corpus entries (operator-level, same for all components)
		var corpusLines []string
		for _, e := range corpusEntries {
			corpusLines = append(corpusLines, fmt.Sprintf(
				"f.Add(uint16(%#04x), uint8(%d), uint16(%d)) // %s",
				e.OpMask, e.FaultType, e.Intensity, e.Label,
			))
		}
		tc.SeedCorpus = strings.Join(corpusLines, "\n\t")

		data.Components = append(data.Components, tc)
	}

	tmpl, err := template.New("fuzz").Parse(fuzzTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.String(), nil
	}

	return string(formatted), nil
}

// pascalCase converts "kserve-controller-manager" to "KserveControllerManager".
func pascalCase(s string) string {
	var result strings.Builder
	upper := true
	for _, r := range s {
		if r == '-' || r == '_' || r == '.' {
			upper = true
			continue
		}
		if upper {
			result.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

const fuzzTemplate = `// Code generated from knowledge model: {{.OperatorName}}. DO NOT EDIT (except reconcilerFactory).

package fuzz_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk/fuzz"
)

// reconcilerFactory creates a reconciler wired to the given client.
// Replace this with your actual reconciler constructor.
func reconcilerFactory(c client.Client) reconcile.Reconciler {
	panic("replace with your reconciler: e.g. return &MyReconciler{client: c}")
}

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = appsv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = coordinationv1.AddToScheme(s)
	_ = rbacv1.AddToScheme(s)
	return s
}
{{range .Components}}
func {{.FuncName}}(f *testing.F) {
	scheme := testScheme()
	req := {{.Request}}

	seeds := []client.Object{
		{{.SeedObjects}}
	}

	{{.SeedCorpus}}

	f.Fuzz(func(t *testing.T, opMask uint16, faultType uint8, intensity uint16) {
		h := fuzz.NewHarness(reconcilerFactory, scheme, req, seeds...)
		{{.Invariants}}
		fc := fuzz.DecodeFaultConfig(opMask, faultType, intensity)
		if err := h.Run(t, fc); err != nil {
			t.Fatal(err)
		}
	})
}
{{end}}`
