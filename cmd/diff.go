package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/config"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Preview local and remote sync actions",
	Long:  `Fetch the latest remote state and show what push or pull would do for each configured agent.`,
	RunE:  runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	cfgPath := resolveConfigPath()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config (run 'negent init' first): %w", err)
	}

	plan, err := loadCurrentPlan(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("loading diff: %w", err)
	}

	printPlanSummary(plan)
	printPlannedChanges("Local changes (push):", plan.Local)
	printPlannedChanges("Remote changes (pull):", plan.Remote)
	return nil
}
