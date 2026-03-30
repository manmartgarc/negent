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

var pushDryRunFlag bool
var pushQuietFlag bool

func init() {
	pushCmd.Flags().BoolVar(&pushDryRunFlag, "dry-run", false, "preview push changes without writing to staging or remote")
	pushCmd.Flags().BoolVar(&pushQuietFlag, "quiet", false, "suppress informational output (errors still go to stderr)")
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	cfgPath := resolveConfigPath()

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

	orch := sync.NewOrchestrator(be, agents)

	if pushDryRunFlag {
		if err := be.Fetch(context.Background()); err != nil {
			return fmt.Errorf("push preview failed: fetching backend: %w", err)
		}
		plan, err := orch.Plan(context.Background(), syncTypes)
		if err != nil {
			return fmt.Errorf("push preview failed: %w", err)
		}
		printPlanSummary(plan)
		printPlannedChanges("Push preview:", plan.Local)
		return nil
	}

	if !pushQuietFlag {
		fmt.Println("Pushing...")
	}
	if err := orch.Push(context.Background(), syncTypes); err != nil {
		return formatSyncOpError("push", "negent push", err)
	}

	if !pushQuietFlag {
		fmt.Println("✓ Push complete")
	}
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

// buildAgents creates agent instances and their sync-type maps from config.
func buildAgents(cfg *config.Config) (map[string]agent.Agent, map[string][]agent.SyncType, error) {
	agents := make(map[string]agent.Agent)
	syncTypes := make(map[string][]agent.SyncType)

	for name, ac := range cfg.Agents {
		ag, err := newAgent(name, ac.Source, ac.Links)
		if err != nil {
			return nil, nil, err
		}
		agents[name] = ag

		types, err := ag.NormalizeSyncTypes(ac.Sync)
		if err != nil {
			return nil, nil, fmt.Errorf("agent %s: %w", name, err)
		}
		syncTypes[name] = types
	}

	return agents, syncTypes, nil
}

// newAgent creates an agent instance by name.
func newAgent(name, sourceDir string, links map[string]string) (agent.Agent, error) {
	expanded := agent.ExpandHome(sourceDir)
	switch name {
	case "claude":
		return agentclaude.New(expanded, links), nil
	default:
		return nil, fmt.Errorf("unsupported agent: %s", name)
	}
}
