package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/config"
)

var (
	configBackendFlag        string
	configRepoFlag           string
	configMachineFlag        string
	configEnableAgentsFlag   []string
	configDisableAgentsFlag  []string
	configNonInteractiveFlag bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage negent configuration",
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit negent configuration",
	Long:  `Edit the negent config interactively by default, or update selected fields non-interactively with flags.`,
	RunE:  runConfigEdit,
}

func init() {
	configEditCmd.Flags().StringVar(&configBackendFlag, "backend", "", "set the backend type")
	configEditCmd.Flags().StringVar(&configRepoFlag, "repo", "", "set the remote repository URL")
	configEditCmd.Flags().StringVar(&configMachineFlag, "machine", "", "set the machine name")
	configEditCmd.Flags().StringSliceVar(&configEnableAgentsFlag, "enable-agent", nil, "enable/configure an agent (repeatable)")
	configEditCmd.Flags().StringSliceVar(&configDisableAgentsFlag, "disable-agent", nil, "disable an agent (repeatable)")
	configEditCmd.Flags().BoolVar(&configNonInteractiveFlag, "non-interactive", false, "disable prompts and require explicit flags")
	configCmd.AddCommand(configEditCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	cfgPath := resolveConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config (run 'negent init' first): %w", err)
	}

	if configNonInteractiveFlag {
		if err := applyNonInteractiveConfigEdits(cfg); err != nil {
			return err
		}
	} else {
		if configBackendFlag != "" {
			cfg.Backend = configBackendFlag
		} else {
			cfg.Backend, err = promptBackendType(cfg.Backend)
			if err != nil {
				return err
			}
		}

		if configRepoFlag != "" {
			cfg.Repo = configRepoFlag
		} else {
			cfg.Repo, err = promptRepoURL(cfg.Repo)
			if err != nil {
				return err
			}
		}

		defaultMachine := cfg.Machine
		if defaultMachine == "" {
			hostname, hostnameErr := os.Hostname()
			if hostnameErr == nil {
				defaultMachine = hostname
			}
		}
		if configMachineFlag != "" {
			cfg.Machine = configMachineFlag
		} else {
			cfg.Machine, err = promptMachineName(defaultMachine)
			if err != nil {
				return err
			}
		}

		selectedAgents := defaultSelectedAgents(cfg.Agents, agent.DetectAgents())
		known := agent.KnownAgents()
		selectedAgents, err = selectAgentsInteractive(known, selectedAgents)
		if err != nil {
			return err
		}

		cfg.Agents, err = buildAgentConfigs(selectedAgents, knownAgentsMap(), cfg.Agents, true)
		if err != nil {
			return err
		}
	}

	if err := config.Save(cfg, cfgPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("✓ Updated config at %s\n", cfgPath)
	return nil
}

func applyNonInteractiveConfigEdits(cfg *config.Config) error {
	if configBackendFlag != "" {
		cfg.Backend = configBackendFlag
	}
	if configRepoFlag != "" {
		cfg.Repo = configRepoFlag
	}
	if configMachineFlag != "" {
		cfg.Machine = configMachineFlag
	}

	known := knownAgentsMap()
	for _, name := range configEnableAgentsFlag {
		if _, ok := cfg.Agents[name]; ok {
			continue
		}
		ka, ok := known[name]
		if !ok {
			return fmt.Errorf("unknown agent %q", name)
		}
		source := ka.SourceDir
		ag, err := newAgent(name, source, nil)
		if err != nil {
			return err
		}
		cfg.Agents[name] = config.AgentConfig{
			Source: source,
			Sync:   defaultSyncTypeStrings(ag),
		}
	}
	for _, name := range configDisableAgentsFlag {
		delete(cfg.Agents, name)
	}
	return nil
}
