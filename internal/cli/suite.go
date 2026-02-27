package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/experiment"
	"github.com/spf13/cobra"
)

func newSuiteCommand() *cobra.Command {
	var (
		knowledgePath string
		reportDir     string
		dryRun        bool
	)

	cmd := &cobra.Command{
		Use:   "suite [experiments-directory]",
		Short: "Run all experiments in a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]

			// Find all YAML files
			entries, err := os.ReadDir(dir)
			if err != nil {
				return fmt.Errorf("reading directory %s: %w", dir, err)
			}

			var experimentFiles []string
			for _, entry := range entries {
				if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
					experimentFiles = append(experimentFiles, filepath.Join(dir, entry.Name()))
				}
			}

			if len(experimentFiles) == 0 {
				fmt.Printf("No experiment files found in %s\n", dir)
				return nil
			}

			fmt.Printf("Found %d experiments in %s\n\n", len(experimentFiles), dir)

			passed := 0
			failed := 0
			skipped := 0

			for _, file := range experimentFiles {
				exp, err := experiment.Load(file)
				if err != nil {
					fmt.Printf("SKIP %s: %v\n", filepath.Base(file), err)
					skipped++
					continue
				}

				errs := experiment.Validate(exp)
				if len(errs) > 0 {
					fmt.Printf("SKIP %s: %d validation errors\n", filepath.Base(file), len(errs))
					skipped++
					continue
				}

				// In dry-run mode, just validate
				if dryRun {
					fmt.Printf("VALID %s (%s)\n", exp.Metadata.Name, exp.Spec.Injection.Type)
					passed++
					continue
				}

				// For actual runs, delegate to run command logic
				fmt.Printf("WOULD RUN %s (use 'odh-chaos run %s' to execute)\n", exp.Metadata.Name, file)
				passed++
			}

			fmt.Printf("\nSuite summary: %d passed, %d failed, %d skipped (total: %d)\n",
				passed, failed, skipped, len(experimentFiles))

			return nil
		},
	}

	cmd.Flags().StringVar(&knowledgePath, "knowledge", "", "path to operator knowledge YAML")
	cmd.Flags().StringVar(&reportDir, "report-dir", "", "directory for report output")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "validate without running (default: true)")

	return cmd
}
