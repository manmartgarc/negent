package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/config"
	syncpkg "github.com/manmart/negent/internal/sync"
)

func resolveConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	return config.DefaultPath()
}

func promptMachineName(defaultValue string) (string, error) {
	value := defaultValue
	err := huh.NewInput().
		Title("Machine name").
		Value(&value).
		Validate(requiredValue("machine name")).
		Run()
	return value, err
}

func promptRepoURL(defaultValue string) (string, error) {
	value := defaultValue
	err := huh.NewInput().
		Title("Git remote URL").
		Placeholder("git@github.com:user/negent-sync.git").
		Value(&value).
		Validate(requiredValue("remote URL")).
		Run()
	return value, err
}

func promptBackendType(defaultValue string) (string, error) {
	value := defaultValue
	if value == "" {
		value = "git"
	}
	err := huh.NewSelect[string]().
		Title("Backend type").
		Options(huh.NewOption("git", "git")).
		Value(&value).
		Run()
	return value, err
}

func requiredValue(name string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
}

func selectAgentsInteractive(options []agent.KnownAgent, selected []string) ([]string, error) {
	if len(options) == 0 {
		return nil, nil
	}

	var promptOptions []huh.Option[string]
	for _, a := range options {
		label := fmt.Sprintf("%s (%s)", a.Name, a.SourceDir)
		promptOptions = append(promptOptions, huh.NewOption(label, a.Name))
	}

	values := append([]string(nil), selected...)
	err := huh.NewMultiSelect[string]().
		Title("Agents to sync").
		Options(promptOptions...).
		Value(&values).
		Run()
	return values, err
}

func defaultSelectedAgents(existing map[string]config.AgentConfig, detected []agent.KnownAgent) []string {
	if len(existing) > 0 {
		names := sortedAgentNames(existing)
		return names
	}
	var names []string
	for _, a := range detected {
		names = append(names, a.Name)
	}
	return names
}

func sortedAgentNames(m map[string]config.AgentConfig) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func buildAgentConfigs(selected []string, detected map[string]agent.KnownAgent, existing map[string]config.AgentConfig, interactive bool) (map[string]config.AgentConfig, error) {
	out := make(map[string]config.AgentConfig, len(selected))
	for _, name := range selected {
		source := ""
		if current, ok := existing[name]; ok && current.Source != "" {
			source = current.Source
		}
		if source == "" {
			source = detected[name].SourceDir
		}

		links := map[string]string(nil)
		if current, ok := existing[name]; ok && len(current.Links) > 0 {
			links = current.Links
		}

		ag, err := newAgent(name, source, links)
		if err != nil {
			return nil, err
		}

		selectedSyncs := defaultSyncTypeStrings(ag)
		if current, ok := existing[name]; ok && len(current.Sync) > 0 {
			selectedSyncs = append([]string(nil), current.Sync...)
		}
		if interactive {
			var options []huh.Option[string]
			for _, syncType := range syncTypeOptions(ag) {
				options = append(options, huh.NewOption(syncType, syncType))
			}
			if err := huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("Select sync types for %s", name)).
				Options(options...).
				Value(&selectedSyncs).
				Run(); err != nil {
				return nil, err
			}
		}

		out[name] = config.AgentConfig{
			Source: source,
			Sync:   selectedSyncs,
			Links:  links,
		}
	}
	return out, nil
}

func knownAgentsMap() map[string]agent.KnownAgent {
	out := make(map[string]agent.KnownAgent)
	for _, a := range agent.KnownAgents() {
		out[a.Name] = a
	}
	return out
}

func detectedAgentsMap() map[string]agent.KnownAgent {
	out := make(map[string]agent.KnownAgent)
	for _, a := range agent.DetectAgents() {
		out[a.Name] = a
	}
	return out
}

func hasAnyNonGitFiles(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() != ".git" {
			return true
		}
	}
	return false
}

func printPlanSummary(plan *syncpkg.SyncPlan) {
	localCounts := countActions(plan.Local)
	remoteCounts := countActions(plan.Remote)
	if len(localCounts) == 0 && len(remoteCounts) == 0 {
		fmt.Println("No pending sync changes.")
		return
	}

	fmt.Println("Pending sync changes:")
	for _, line := range formatCounts("local", localCounts) {
		fmt.Println(line)
	}
	for _, line := range formatCounts("remote", remoteCounts) {
		fmt.Println(line)
	}
}

func printPlannedChanges(title string, changes []syncpkg.PlannedChange) {
	if len(changes) == 0 {
		return
	}
	fmt.Println(title)
	sorted := make([]syncpkg.PlannedChange, len(changes))
	copy(sorted, changes)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Agent == sorted[j].Agent {
			if sorted[i].Action == sorted[j].Action {
				return sorted[i].Path < sorted[j].Path
			}
			return sorted[i].Action < sorted[j].Action
		}
		return sorted[i].Agent < sorted[j].Agent
	})
	for _, change := range sorted {
		fmt.Printf("  %-13s %s:%s\n", string(change.Action), change.Agent, change.Path)
	}
}

func countActions(changes []syncpkg.PlannedChange) map[syncpkg.PlanAction]int {
	counts := map[syncpkg.PlanAction]int{}
	for _, change := range changes {
		counts[change.Action]++
	}
	return counts
}

func formatCounts(prefix string, counts map[syncpkg.PlanAction]int) []string {
	var actions []string
	for action := range counts {
		actions = append(actions, string(action))
	}
	sort.Strings(actions)
	var lines []string
	for _, action := range actions {
		lines = append(lines, fmt.Sprintf("  %s %-13s %d", prefix, action, counts[syncpkg.PlanAction(action)]))
	}
	return lines
}

func confirmDestructiveAction(title string) (bool, error) {
	confirmed := false
	err := huh.NewConfirm().
		Title(title).
		Value(&confirmed).
		Run()
	return confirmed, err
}

func findSourceDirForAgent(cfg *config.Config, agentName string) string {
	if ac, ok := cfg.Agents[agentName]; ok && ac.Source != "" {
		return agent.ExpandHome(ac.Source)
	}
	if ka, ok := knownAgentsMap()[agentName]; ok {
		return agent.ExpandHome(ka.SourceDir)
	}
	return ""
}

func nextSuggestedCommand(bePath string) string {
	if hasAnyNonGitFiles(bePath) {
		return "negent pull --dry-run"
	}
	return "negent push --dry-run"
}

func loadCurrentPlan(ctx context.Context, cfg *config.Config) (*syncpkg.SyncPlan, error) {
	be, err := newBackend(cfg)
	if err != nil {
		return nil, err
	}
	if err := be.Fetch(ctx); err != nil {
		return nil, fmt.Errorf("fetching backend: %w", err)
	}
	agents, syncTypes, err := buildAgents(cfg)
	if err != nil {
		return nil, err
	}
	orch := syncpkg.NewOrchestrator(be, agents)
	return orch.Plan(ctx, syncTypes)
}
