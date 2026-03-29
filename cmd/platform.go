package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var currentGOOS = runtime.GOOS

func checkPlatformSupport(_ *cobra.Command, _ []string) error {
	if currentGOOS == "windows" {
		return fmt.Errorf("windows is unsupported; negent supports Linux and macOS only")
	}
	return nil
}
