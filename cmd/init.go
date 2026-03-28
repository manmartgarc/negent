package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/backend"
	gitbackend "github.com/manmart/negent/internal/backend/git"
	"github.com/manmart/negent/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize negent on this machine",
	Long:  `Interactive first-time setup: configure backend, machine name, and agents to sync.`,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	if config.Exists(cfgPath) {
		return fmt.Errorf("config already exists at %s — use 'negent add' to add agents, or delete the config to re-init", cfgPath)
	}

	// Step 1: Backend selection
	var backendType string
	err := huh.NewSelect[string]().
		Title("Backend type").
		Options(
			huh.NewOption("git", "git"),
		).
		Value(&backendType).
		Run()
	if err != nil {
		return err
	}

	// Step 2: Backend-specific config
	var remoteURL string
	if backendType == "git" {
		err = huh.NewInput().
			Title("Git remote URL").
			Placeholder("git@github.com:user/negent-sync.git").
			Value(&remoteURL).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("remote URL is required")
				}
				return nil
			}).
			Run()
		if err != nil {
			return err
		}
	}

	// Step 3: Machine name
	hostname, _ := os.Hostname()
	machineName := hostname
	err = huh.NewInput().
		Title("Machine name").
		Value(&machineName).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("machine name is required")
			}
			return nil
		}).
		Run()
	if err != nil {
		return err
	}

	// Step 4: Agent detection
	detected := agent.DetectAgents()
	if len(detected) == 0 {
		fmt.Println("No known AI assistant configs detected. You can add agents later with 'negent add'.")
	}

	var selectedAgents []string
	if len(detected) > 0 {
		var options []huh.Option[string]
		for _, a := range detected {
			label := fmt.Sprintf("%s (%s)", a.Name, a.SourceDir)
			options = append(options, huh.NewOption(label, a.Name))
		}

		err = huh.NewMultiSelect[string]().
			Title("Detected agents — which to sync?").
			Options(options...).
			Value(&selectedAgents).
			Run()
		if err != nil {
			return err
		}
	}

	// Step 5: Build config
	cfg := &config.Config{
		Backend: backendType,
		Repo:    remoteURL,
		Machine: machineName,
		Agents:  make(map[string]config.AgentConfig),
	}

	// For each selected agent, use default categories
	for _, name := range selectedAgents {
		var src string
		for _, ka := range detected {
			if ka.Name == name {
				src = ka.SourceDir
				break
			}
		}
		defaults := defaultCategoriesFor(name)
		cfg.Agents[name] = config.AgentConfig{
			Source: src,
			Sync:   defaults,
		}
	}

	// Step 6: Initialize backend and verify access
	var be backend.Backend
	switch backendType {
	case "git":
		be = gitbackend.New(remoteURL, gitbackend.DefaultStagingDir())
	default:
		return fmt.Errorf("unsupported backend: %s", backendType)
	}

	fmt.Println("Verifying backend access...")
	if err := be.Init(context.Background(), backend.BackendConfig{"remote": remoteURL}); err != nil {
		return fmt.Errorf("backend init failed: %w", err)
	}

	// Step 7: Write config
	if err := config.Save(cfg, cfgPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	// Step 8: Summary
	fmt.Printf("\n✓ Config written to %s\n", cfgPath)
	fmt.Printf("✓ Backend: %s\n", backendType)
	fmt.Printf("✓ Machine: %s\n", machineName)
	if len(selectedAgents) > 0 {
		fmt.Printf("✓ Agents: %v\n", selectedAgents)
	}
	fmt.Println("✓ Initial pull complete")

	return nil
}

// defaultCategoriesFor returns the default sync categories for a known agent.
func defaultCategoriesFor(_ string) []string {
	// All known agents default to config + custom-code + memory
	return []string{"config", "custom-code", "memory"}
}
