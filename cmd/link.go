package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link <agent> <remote-project> <local-path>",
	Short: "Manually link a remote project to a local path",
	Long:  `Resolve an unmatched project directory by explicitly mapping it to a local path. Used when automatic matching fails.`,
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement in Phase 5
		fmt.Printf("negent link: not yet implemented\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(linkCmd)
}
