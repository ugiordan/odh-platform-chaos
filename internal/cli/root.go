package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "odh-chaos",
		Short: "Chaos engineering framework for OpenDataHub operators",
		Long: `ODH Platform Chaos tests operator reconciliation semantics.
It validates that operators recover managed resources correctly after
fault injection, not just that pods restart.`,
	}

	cmd.PersistentFlags().String("kubeconfig", "", "path to kubeconfig file")
	cmd.PersistentFlags().String("namespace", "opendatahub", "target namespace")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")

	cmd.AddCommand(
		newValidateCommand(),
		newRunCommand(),
		newCleanCommand(),
		newInitCommand(),
		newAnalyzeCommand(),
		newSuiteCommand(),
		newReportCommand(),
	)

	return cmd
}
