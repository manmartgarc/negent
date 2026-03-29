package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/backend"
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

	agents, syncTypes, err := buildAgents(cfg)
	if err != nil {
		return err
	}

	stagingDir := be.StagingDir()

	for name, ag := range agents {
		agentStagingDir := filepath.Join(stagingDir, name)
		changes, err := ag.Diff(agentStagingDir, syncTypes[name])
		if err != nil {
			fmt.Printf("%s: error: %v\n", name, err)
			continue
		}

		if len(changes) == 0 {
			fmt.Printf("%s: up to date\n", name)
			continue
		}

		fmt.Printf("%s:\n", name)
		for _, ch := range changes {
			var prefix string
			switch ch.Kind {
			case backend.ChangeAdded:
				prefix = "  New:      "
			case backend.ChangeModified:
				prefix = "  Modified: "
			case backend.ChangeDeleted:
				prefix = "  Deleted:  "
			}
			fmt.Printf("%s%s\n", prefix, ch.Path)
		}
	}

	return nil
}
