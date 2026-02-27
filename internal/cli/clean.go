package cli

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/spf13/cobra"
)

func newCleanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove all chaos artifacts from the cluster (emergency stop)",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, _ := cmd.Flags().GetString("namespace")

			cfg, err := config.GetConfig()
			if err != nil {
				return fmt.Errorf("getting kubeconfig: %w", err)
			}

			k8sClient, err := client.New(cfg, client.Options{})
			if err != nil {
				return fmt.Errorf("creating k8s client: %w", err)
			}

			ctx := context.Background()
			cleaned := 0

			// Clean NetworkPolicies with chaos label
			policies := &networkingv1.NetworkPolicyList{}
			if err := k8sClient.List(ctx, policies,
				client.InNamespace(namespace),
				client.MatchingLabels{"app.kubernetes.io/managed-by": "odh-chaos"},
			); err != nil {
				return fmt.Errorf("listing chaos NetworkPolicies: %w", err)
			}

			for i := range policies.Items {
				fmt.Printf("Deleting NetworkPolicy %s/%s\n", policies.Items[i].Namespace, policies.Items[i].Name)
				if err := k8sClient.Delete(ctx, &policies.Items[i]); err != nil {
					fmt.Printf("  Warning: %v\n", err)
				} else {
					cleaned++
				}
			}

			if cleaned == 0 {
				fmt.Println("No chaos artifacts found.")
			} else {
				fmt.Printf("Cleaned %d chaos artifacts.\n", cleaned)
			}

			return nil
		},
	}
}
