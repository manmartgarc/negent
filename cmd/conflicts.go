package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/config"
	syncpkg "github.com/manmart/negent/internal/sync"
)

var conflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "List and resolve sync conflicts",
	RunE:  runConflicts,
}

var (
	conflictsKeepRemote bool
	conflictsList       bool
)

func init() {
	conflictsCmd.Flags().BoolVar(&conflictsKeepRemote, "keep-remote", false, "overwrite all local files with remote versions")
	conflictsCmd.Flags().BoolVar(&conflictsList, "list", false, "list conflicts without resolving")
	rootCmd.AddCommand(conflictsCmd)
}

func runConflicts(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(resolveConfigPath())
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

	orch := syncpkg.NewOrchestrator(be, agents)
	conflicts, err := orch.Conflicts(syncTypes)
	if err != nil {
		return fmt.Errorf("detecting conflicts: %w", err)
	}

	if len(conflicts) == 0 {
		fmt.Println("No conflicts.")
		return nil
	}

	if conflictsList {
		for _, c := range conflicts {
			fmt.Printf("  CONFLICT %s:%s\n", c.Agent, c.RelPath)
		}
		return nil
	}

	if conflictsKeepRemote {
		for _, c := range conflicts {
			if err := copyFileFromTo(c.StagingPath, c.LocalPath); err != nil {
				return fmt.Errorf("overwriting %s: %w", c.RelPath, err)
			}
			fmt.Printf("  took remote: %s:%s\n", c.Agent, c.RelPath)
		}
		return nil
	}

	// Interactive mode.
	return resolveInteractive(conflicts)
}

func resolveInteractive(conflicts []syncpkg.ConflictInfo) error {
	for i, c := range conflicts {
		fmt.Printf("\n[%d/%d] %s:%s\n", i+1, len(conflicts), c.Agent, c.RelPath)
		for {
			var choice string
			err := huh.NewSelect[string]().
				Title("Resolve conflict").
				Options(
					huh.NewOption("Keep local", "l"),
					huh.NewOption("Take remote", "r"),
					huh.NewOption("Show diff", "d"),
					huh.NewOption("Skip", "s"),
					huh.NewOption("Quit", "q"),
				).
				Value(&choice).
				Run()
			if err != nil {
				return err
			}
			switch choice {
			case "l":
				fmt.Printf("  kept local: %s\n", c.RelPath)
			case "r":
				if err := copyFileFromTo(c.StagingPath, c.LocalPath); err != nil {
					return fmt.Errorf("overwriting %s: %w", c.RelPath, err)
				}
				fmt.Printf("  took remote: %s\n", c.RelPath)
			case "d":
				printDiff(c.LocalPath, c.StagingPath)
				continue
			case "s":
				fmt.Printf("  skipped: %s\n", c.RelPath)
			case "q":
				return nil
			}
			break
		}
	}
	return nil
}

func printDiff(localPath, stagingPath string) {
	// Use unified diff via the system diff command for readable output.
	cmd := exec.Command("diff", "-u", "--label", "local", "--label", "remote", localPath, stagingPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run() // exit code 1 means files differ, which is expected
}

func copyFileFromTo(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}
