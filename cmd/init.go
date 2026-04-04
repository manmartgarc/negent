package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/backend"
	gitbackend "github.com/manmart/negent/internal/backend/git"
	"github.com/manmart/negent/internal/config"
)

var (
	initBackendFlag        string
	initRepoFlag           string
	initMachineFlag        string
	initAgentsFlag         []string
	initNonInteractiveFlag bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize negent on this machine",
	Long:  `First-time setup: configure backend, machine name, and the agents to sync on this machine.`,
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVar(&initBackendFlag, "backend", "", "backend type to configure")
	initCmd.Flags().StringVar(&initRepoFlag, "repo", "", "remote repository URL")
	initCmd.Flags().StringVar(&initMachineFlag, "machine", "", "machine name")
	initCmd.Flags().StringSliceVar(&initAgentsFlag, "agent", nil, "agent to configure (repeatable)")
	initCmd.Flags().BoolVar(&initNonInteractiveFlag, "non-interactive", false, "disable prompts and require flags/defaults")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cfgPath := resolveConfigPath()

	if config.Exists(cfgPath) {
		return fmt.Errorf("config already exists at %s — use 'negent add' to add agents, or delete the config to re-init", cfgPath)
	}

	backendType := initBackendFlag
	var err error
	if backendType == "" && !initNonInteractiveFlag {
		backendType, err = promptBackendType("git")
		if err != nil {
			return err
		}
	}
	if backendType == "" {
		backendType = "git"
	}

	remoteURL := initRepoFlag
	if backendType == "git" {
		if remoteURL == "" && !initNonInteractiveFlag {
			remoteURL, err = promptRepoURL("")
			if err != nil {
				return err
			}
		}
		if remoteURL == "" {
			return fmt.Errorf("--repo is required in non-interactive mode")
		}
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "this-machine"
	}
	machineName := initMachineFlag
	if machineName == "" && !initNonInteractiveFlag {
		machineName, err = promptMachineName(hostname)
		if err != nil {
			return err
		}
	}
	if machineName == "" {
		machineName = hostname
	}

	detected := agent.DetectAgents()
	if len(detected) == 0 {
		fmt.Println("No known AI assistant configs detected. You can add agents later with 'negent add'.")
	}

	selectedAgents := append([]string(nil), initAgentsFlag...)
	if len(selectedAgents) == 0 {
		selectedAgents = defaultSelectedAgents(nil, detected)
	}
	if !initNonInteractiveFlag {
		selectedAgents, err = selectAgentsInteractive(detected, selectedAgents)
		if err != nil {
			return err
		}
	}

	detectedMap := detectedAgentsMap()
	agentConfigs, err := buildAgentConfigs(selectedAgents, detectedMap, nil, !initNonInteractiveFlag)
	if err != nil {
		return err
	}

	cfg := &config.Config{
		Backend: backendType,
		Repo:    remoteURL,
		Machine: machineName,
		Agents:  agentConfigs,
	}

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

	if err := config.Save(cfg, cfgPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\n✓ Config written to %s\n", cfgPath)
	fmt.Printf("✓ Backend: %s\n", backendType)
	fmt.Printf("✓ Machine: %s\n", machineName)
	if len(selectedAgents) > 0 {
		fmt.Printf("✓ Agents: %v\n", selectedAgents)
	}
	fmt.Printf("Next: %s\n", nextSuggestedCommand(be.StagingDir()))

	return nil
}
