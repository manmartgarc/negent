package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status for configured agents",
	Long:  `Display the diff between local agent directories and the remote backend.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement in Phase 5
		fmt.Println("negent status: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
