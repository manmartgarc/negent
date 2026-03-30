package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/hooks"
)

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Manage automatic sync hooks",
	Long:  `Install or remove hooks that automatically run 'negent pull' at session start and 'negent push' at session end.`,
}

var autoAgentFlag string

var autoEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Install auto-sync hooks",
	RunE:  runAutoEnable,
}

var autoDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Remove auto-sync hooks",
	RunE:  runAutoDisable,
}

var autoStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether auto-sync hooks are installed",
	RunE:  runAutoStatus,
}

func init() {
	autoEnableCmd.Flags().StringVar(&autoAgentFlag, "agent", "claude", "agent CLI to configure hooks for")
	autoDisableCmd.Flags().StringVar(&autoAgentFlag, "agent", "claude", "agent CLI to remove hooks from")
	autoStatusCmd.Flags().StringVar(&autoAgentFlag, "agent", "claude", "agent CLI to check hook status for")
	autoCmd.AddCommand(autoEnableCmd, autoDisableCmd, autoStatusCmd)
	rootCmd.AddCommand(autoCmd)
}

func runAutoEnable(cmd *cobra.Command, args []string) error {
	switch autoAgentFlag {
	case "claude":
		negentBin, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolving negent binary path: %w", err)
		}
		if err := hooks.InstallClaude(hooks.DefaultClaudeSettingsPath(), negentBin); err != nil {
			return fmt.Errorf("installing hooks: %w", err)
		}
		fmt.Println("Auto-sync enabled. negent will pull on session start and push on session end.")
		return nil
	default:
		return fmt.Errorf("unsupported agent %q (supported: claude)", autoAgentFlag)
	}
}

func runAutoDisable(cmd *cobra.Command, args []string) error {
	switch autoAgentFlag {
	case "claude":
		if err := hooks.UninstallClaude(hooks.DefaultClaudeSettingsPath()); err != nil {
			return fmt.Errorf("removing hooks: %w", err)
		}
		fmt.Println("Auto-sync disabled.")
		return nil
	default:
		return fmt.Errorf("unsupported agent %q (supported: claude)", autoAgentFlag)
	}
}

func runAutoStatus(cmd *cobra.Command, args []string) error {
	switch autoAgentFlag {
	case "claude":
		enabled, err := hooks.StatusClaude(hooks.DefaultClaudeSettingsPath())
		if err != nil {
			return fmt.Errorf("reading hook status: %w", err)
		}
		if enabled {
			fmt.Println("Auto-sync: enabled (SessionStart + Stop)")
		} else {
			fmt.Println("Auto-sync: disabled")
		}
		return nil
	default:
		return fmt.Errorf("unsupported agent %q (supported: claude)", autoAgentFlag)
	}
}
