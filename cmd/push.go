package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/agent"
	agentclaude "github.com/manmart/negent/internal/agent/claude"
	"github.com/manmart/negent/internal/backend"
	gitbackend "github.com/manmart/negent/internal/backend/git"
	"github.com/manmart/negent/internal/config"
	"github.com/manmart/negent/internal/sync"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local agent configs to the remote",
	Long:  `Collect files from configured agents, stage them, and push to the remote backend.`,
	RunE:  runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
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

	fmt.Println("Pushing...")
	if err := orch.Push(context.Background(), categories); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	fmt.Println("✓ Push complete")
	return nil
}

// newBackend creates a backend from the config.
func newBackend(cfg *config.Config) (backend.Backend, error) {
	switch cfg.Backend {
	case "git":
		return gitbackend.New(cfg.Repo, gitbackend.DefaultStagingDir()), nil
	default:
		return nil, fmt.Errorf("unsupported backend: %s", cfg.Backend)
	}
}

// buildAgents creates agent instances and their category maps from config.
func buildAgents(cfg *config.Config) (map[string]agent.Agent, map[string][]agent.Category, error) {
	agents := make(map[string]agent.Agent)
	categories := make(map[string][]agent.Category)

	for name, ac := range cfg.Agents {
		ag, err := newAgent(name, ac.Source)
		if err != nil {
			return nil, nil, err
		}
		agents[name] = ag

		var cats []agent.Category
		for _, s := range ac.Sync {
			cats = append(cats, agent.Category(s))
		}
		categories[name] = cats
	}

	return agents, categories, nil
}

// newAgent creates an agent instance by name.
func newAgent(name, sourceDir string) (agent.Agent, error) {
	expanded := agent.ExpandHome(sourceDir)
	switch name {
	case "claude":
		return agentclaude.New(expanded), nil
	default:
		return nil, fmt.Errorf("unsupported agent: %s", name)
	}
}
