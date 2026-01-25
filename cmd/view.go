package cmd

import (
	"fmt"

	"github.com/jeduardo/fic/internal/fic"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "View scan contents",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, _ := cmd.Flags().GetString("state")
		onlyDone, _ := cmd.Flags().GetBool("only-done")
		format, _ := cmd.Flags().GetString("format")

		if state == "" {
			return fmt.Errorf("--state is required")
		}

		return fic.RunView(state, onlyDone, format)
	},
}

func init() {
	rootCmd.AddCommand(viewCmd)

	viewCmd.Flags().String("state", "", "state file (.fic)")
	viewCmd.Flags().Bool("only-done", false, "show only completed entries")
	viewCmd.Flags().String("format", "text", "output format: text or json")
}
