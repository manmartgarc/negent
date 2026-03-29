package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/config"
)

var addCmd = &cobra.Command{
	Use:   "add <agent>",
	Short: "Register an agent for syncing",
	Long:  `Add a new AI assistant agent to sync. Detects the agent's config directory and configures default sync types.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

var addSourceFlag string

func init() {
	addCmd.Flags().StringVar(&addSourceFlag, "source", "", "override the agent's source directory")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	agentName := args[0]

	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config (run 'negent init' first): %w", err)
	}

	if _, exists := cfg.Agents[agentName]; exists {
		return fmt.Errorf("agent %q is already configured", agentName)
	}

	// Resolve source directory
	sourceDir := addSourceFlag
	if sourceDir == "" {
		for _, ka := range agent.KnownAgents() {
			if ka.Name == agentName {
				sourceDir = ka.SourceDir
				break
			}
		}
	}
	if sourceDir == "" {
		return fmt.Errorf("unknown agent %q — use --source to specify the config directory", agentName)
	}

	// Verify source exists
	expanded := agent.ExpandHome(sourceDir)
	info, err := os.Stat(expanded)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("agent source directory not found: %s", expanded)
	}

	var selectedCats []string
	if ag, err := newAgent(agentName, sourceDir, nil); err == nil {
		// Agent has a full implementation — offer interactive sync type selection.
		defaults := defaultSyncTypeStrings(ag)
		allSyncTypes := syncTypeOptions(ag)

		var options []huh.Option[string]
		for _, syncType := range allSyncTypes {
			options = append(options, huh.NewOption(syncType, syncType))
		}

		selectedCats = make([]string, len(defaults))
		copy(selectedCats, defaults)

		if err := huh.NewMultiSelect[string]().
			Title(fmt.Sprintf("Select sync types for %s", agentName)).
			Options(options...).
			Value(&selectedCats).
			Run(); err != nil {
			return err
		}
	}

	cfg.Agents[agentName] = config.AgentConfig{
		Source: sourceDir,
		Sync:   selectedCats,
	}

	if err := config.Save(cfg, cfgPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("✓ Added %s (%s)\n", agentName, sourceDir)
	fmt.Printf("✓ Syncing: %v\n", selectedCats)
	return nil
}
