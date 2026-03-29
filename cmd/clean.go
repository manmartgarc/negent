package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/config"
)

var (
	cleanYesFlag            bool
	cleanNonInteractiveFlag bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean <agent>",
	Short: "Delete local agent configuration on this machine",
	Long:  `Remove the local configuration directory for a configured agent on this machine. The agent stays in negent config so the normal recovery flow is to pull it again from the remote.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanYesFlag, "yes", false, "confirm deletion without prompting")
	cleanCmd.Flags().BoolVar(&cleanNonInteractiveFlag, "non-interactive", false, "disable prompts")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	cfgPath := resolveConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config (run 'negent init' first): %w", err)
	}

	agentName := args[0]
	sourceDir := findSourceDirForAgent(cfg, agentName)
	if sourceDir == "" {
		return fmt.Errorf("agent %q is not configured", agentName)
	}
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		fmt.Printf("%s is already clean (%s does not exist). Next step: negent pull --dry-run\n", agentName, sourceDir)
		return nil
	}

	if !cleanYesFlag {
		if cleanNonInteractiveFlag {
			return fmt.Errorf("--yes is required in non-interactive mode")
		}
		confirmed, err := confirmDestructiveAction(fmt.Sprintf("Delete %s local data at %s? You can restore it with negent pull.", agentName, sourceDir))
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Clean cancelled.")
			return nil
		}
	}

	if err := os.RemoveAll(sourceDir); err != nil {
		return fmt.Errorf("removing %s: %w", sourceDir, err)
	}

	fmt.Printf("✓ Removed %s local data from %s\n", agentName, sourceDir)
	fmt.Println("Next: negent pull --dry-run")
	return nil
}
