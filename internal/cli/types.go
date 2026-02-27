package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTypesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "types",
		Short: "List available injection types",
		Run: func(cmd *cobra.Command, args []string) {
			types := []struct {
				name   string
				desc   string
				danger string
			}{
				{"PodKill", "Delete pods matching a label selector", "low"},
				{"NetworkPartition", "Create deny-all NetworkPolicy", "medium"},
				{"ConfigDrift", "Modify ConfigMap or Secret data", "low"},
				{"CRDMutation", "Mutate a field on any Kubernetes resource", "medium"},
				{"FinalizerBlock", "Add a blocking finalizer to a resource", "medium"},
				{"WebhookDisrupt", "Change webhook failure policy", "high"},
				{"RBACRevoke", "Revoke RBAC binding subjects", "high"},
			}
			fmt.Println("Available injection types:")
			fmt.Println()
			for _, t := range types {
				fmt.Printf("  %-20s [%s] %s\n", t.name, t.danger, t.desc)
			}
		},
	}
}
