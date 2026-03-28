package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/config"
)

var linkCmd = &cobra.Command{
	Use:   "link <agent> <remote-project> <local-path>",
	Short: "Manually link a remote project to a local path",
	Long:  `Resolve an unmatched project directory by explicitly mapping it to a local path. Used when automatic matching fails.`,
	Args:  cobra.ExactArgs(3),
	RunE:  runLink,
}

func init() {
	rootCmd.AddCommand(linkCmd)
}

func runLink(cmd *cobra.Command, args []string) error {
	agentName := args[0]
	remoteProject := args[1]
	localPath := args[2]

	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config (run 'negent init' first): %w", err)
	}

	ac, ok := cfg.Agents[agentName]
	if !ok {
		return fmt.Errorf("agent %q is not configured — run 'negent add %s' first", agentName, agentName)
	}

	if ac.Links == nil {
		ac.Links = make(map[string]string)
	}
	ac.Links[remoteProject] = localPath
	cfg.Agents[agentName] = ac

	if err := config.Save(cfg, cfgPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("✓ Linked %s project %q → %s\n", agentName, remoteProject, localPath)
	fmt.Println("  Will sync on next pull.")
	return nil
}
