package cmd

import (
	"fmt"
	"os"

	"github.com/clcollins/srepd/pkg/tui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update srepd to the latest release",
	Long: `Update srepd to the latest GitHub release in place.

Downloads the latest release binary for the current OS and architecture
from https://github.com/clcollins/srepd/releases, extracts it, and
replaces the current binary. Restart srepd after updating.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := tui.RunSelfUpdate(); err != nil {
			fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
