package cmd

import (
	"fmt"
	"os"

	"github.com/manmart/negent/internal/config"
	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:               "negent",
	Short:             "Sync AI assistant configs across machines",
	Long:              `negent (negative entropy) keeps your AI coding assistant configurations in sync across machines using a git-backed (or other) remote store.`,
	PersistentPreRunE: checkPlatformSupport,
}

// RootCommand exposes the root command tree for internal tooling.
func RootCommand() *cobra.Command {
	return rootCmd
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default: %s)", config.DefaultPath()))
}
