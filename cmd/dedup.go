package cmd

import (
	"fmt"

	"github.com/jeduardo/fic/internal/fic"
	"github.com/spf13/cobra"
)

var dedupCmd = &cobra.Command{
	Use:   "dedup <state>",
	Short: "Find duplicate files in a state file",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		state, _ := cmd.Flags().GetString("state")
		format, _ := cmd.Flags().GetString("format")
		out, _ := cmd.Flags().GetString("out")

		if state == "" && len(args) == 1 {
			state = args[0]
		}
		if state == "" {
			return fmt.Errorf("state path is required")
		}

		return fic.RunDedup(state, format, out)
	},
}

func init() {
	rootCmd.AddCommand(dedupCmd)

	dedupCmd.Flags().String("state", "", "state file (.fic)")
	dedupCmd.Flags().String("format", "text", "output format: text or json")
	dedupCmd.Flags().String("out", "", "output file (optional)")
}
