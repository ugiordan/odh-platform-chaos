package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opendatahub-io/operator-chaos/pkg/reporter"
	"github.com/spf13/cobra"
)

func newReportCommand() *cobra.Command {
	var (
		format string
		output string
	)

	cmd := &cobra.Command{
		Use:   "report <results-directory>",
		Short: "Generate reports from experiment results",
		Long:  "Reads JSON experiment results from a directory and generates reports in the specified format.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]

			reports, err := loadReports(dir)
			if err != nil {
				return err
			}

			if len(reports) == 0 {
				fmt.Println("No experiment reports found.")
				return nil
			}

			return writeReport(format, output, dir, reports)
		},
	}

	cmd.Flags().StringVar(&format, "format", "summary", "output format (summary, json, junit, html, markdown)")
	cmd.Flags().StringVar(&output, "output", "", "output file path (default: stdout for summary/markdown, auto-named file for json/junit/html)")

	return cmd
}

func loadReports(dir string) ([]reporter.ExperimentReport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading results directory %s: %w", dir, err)
	}

	var reports []reporter.ExperimentReport
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		// Skip consolidated report files to avoid double-counting
		if entry.Name() == "report.json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", entry.Name(), err)
			continue
		}

		var report reporter.ExperimentReport
		if err := json.Unmarshal(data, &report); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", entry.Name(), err)
			continue
		}

		reports = append(reports, report)
	}

	return reports, nil
}

func writeReport(format, output, inputDir string, reports []reporter.ExperimentReport) error {
	switch format {
	case "summary":
		return writeSummaryReport(output, reports)
	case "json":
		return writeFormattedReport(&reporter.ConsolidatedJSONReporter{}, output, inputDir, "report.json", reports)
	case "junit":
		return writeJUnitReport(output, inputDir, reports)
	case "html":
		return writeFormattedReport(reporter.NewHTMLReporter(Version), output, inputDir, "report.html", reports)
	case "markdown":
		return writeFormattedReport(&reporter.MarkdownReporter{}, output, inputDir, "", reports)
	default:
		return fmt.Errorf("unknown format %q; supported: summary, json, junit, html, markdown", format)
	}
}

// writeFormattedReport writes using a Reporter implementation.
// When output is empty and defaultFilename is empty, writes to stdout (markdown).
// When output is empty and defaultFilename is set, writes to inputDir/defaultFilename (json, html).
func writeFormattedReport(r reporter.Reporter, output, inputDir, defaultFilename string, reports []reporter.ExperimentReport) error {
	w := os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer func() { _ = f.Close() }()
		w = f
		fmt.Fprintf(os.Stderr, "Writing report to %s\n", output)
	} else if defaultFilename != "" {
		outPath := filepath.Join(inputDir, defaultFilename)
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer func() { _ = f.Close() }()
		w = f
		fmt.Fprintf(os.Stderr, "Writing report to %s\n", outPath)
	}
	return r.WriteReport(w, reports)
}

func writeSummaryReport(output string, reports []reporter.ExperimentReport) error {
	w := os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer func() { _ = f.Close() }()
		w = f
	}

	_, _ = fmt.Fprintf(w, "Chaos Engineering Report (%d experiments)\n", len(reports))
	_, _ = fmt.Fprintln(w, strings.Repeat("=", 80))
	_, _ = fmt.Fprintf(w, "  %-30s  %-14s  %-12s  %s\n", "EXPERIMENT", "VERDICT", "RECOVERY", "DEVIATIONS")
	_, _ = fmt.Fprintln(w, strings.Repeat("-", 80))
	for _, r := range reports {
		recoveryStr := r.Evaluation.RecoveryTime.Round(time.Second).String()
		deviationCount := len(r.Evaluation.Deviations)
		if w == os.Stdout {
			_, _ = fmt.Fprintf(w, "  %-30s  %s  %-12s  %d\n",
				r.Experiment,
				paddedColorVerdict(string(r.Evaluation.Verdict), 14),
				recoveryStr,
				deviationCount)
		} else {
			_, _ = fmt.Fprintf(w, "  %-30s  %-14s  %-12s  %d\n",
				r.Experiment,
				r.Evaluation.Verdict,
				recoveryStr,
				deviationCount)
		}
	}
	_, _ = fmt.Fprintln(w, strings.Repeat("=", 80))
	return nil
}

func writeJUnitReport(output, inputDir string, reports []reporter.ExperimentReport) error {
	outPath := output
	if outPath == "" {
		outPath = filepath.Join(inputDir, "report.xml")
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() { _ = f.Close() }()
	fmt.Fprintf(os.Stderr, "Writing JUnit report to %s\n", outPath)

	r := reporter.NewJUnitReporter(f)
	return r.WriteSuite("operator-chaos-results", reports)
}
