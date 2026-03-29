package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/config"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status for configured agents",
	Long:  `Display the diff between local agent directories and the remote backend.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfgPath := resolveConfigPath()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config (run 'negent init' first): %w", err)
	}

	plan, err := loadCurrentPlan(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("loading sync status: %w", err)
	}
	printPlanSummary(plan)
	return nil
}
