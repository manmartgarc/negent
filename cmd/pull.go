package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/config"
	"github.com/manmart/negent/internal/sync"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull remote agent configs to this machine",
	Long:  `Fetch the latest from the remote backend and place files into local agent directories.`,
	RunE:  runPull,
}

var pullDryRunFlag bool

func init() {
	pullCmd.Flags().BoolVar(&pullDryRunFlag, "dry-run", false, "preview pull changes without writing local files")
	rootCmd.AddCommand(pullCmd)
}

func runPull(cmd *cobra.Command, args []string) error {
	cfgPath := resolveConfigPath()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config (run 'negent init' first): %w", err)
	}

	be, err := newBackend(cfg)
	if err != nil {
		return err
	}

	agents, syncTypes, err := buildAgents(cfg)
	if err != nil {
		return err
	}

	orch := sync.NewOrchestrator(be, agents)

	if pullDryRunFlag {
		if err := be.Fetch(context.Background()); err != nil {
			return fmt.Errorf("pull preview failed: fetching backend: %w", err)
		}
		plan, err := orch.Plan(context.Background(), syncTypes)
		if err != nil {
			return fmt.Errorf("pull preview failed: %w", err)
		}
		printPlanSummary(plan)
		printPlannedChanges("Pull preview:", plan.Remote)
		return nil
	}

	fmt.Println("Pulling...")
	if err := orch.Pull(context.Background(), syncTypes); err != nil {
		return formatSyncOpError("pull", "negent pull", err)
	}

	fmt.Println("✓ Pull complete")
	return nil
}
