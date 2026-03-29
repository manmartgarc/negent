package cmd

import "github.com/manmart/negent/internal/agent"

func syncTypeOptions(ag agent.Agent) []string {
	specs := ag.SupportedSyncTypes()
	options := make([]string, 0, len(specs))
	for _, spec := range specs {
		options = append(options, string(spec.ID))
	}
	return options
}

func defaultSyncTypeStrings(ag agent.Agent) []string {
	defaults := ag.DefaultSyncTypes()
	out := make([]string, 0, len(defaults))
	for _, syncType := range defaults {
		out = append(out, string(syncType))
	}
	return out
}
