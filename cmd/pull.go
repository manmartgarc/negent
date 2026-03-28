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

func init() {
	rootCmd.AddCommand(pullCmd)
}

func runPull(cmd *cobra.Command, args []string) error {
	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config (run 'negent init' first): %w", err)
	}

	be, err := newBackend(cfg)
	if err != nil {
		return err
	}

	agents, categories, err := buildAgents(cfg)
	if err != nil {
		return err
	}

	orch := sync.NewOrchestrator(be, agents)

	fmt.Println("Pulling...")
	if err := orch.Pull(context.Background(), categories); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	fmt.Println("✓ Pull complete")
	return nil
}
