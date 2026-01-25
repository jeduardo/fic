package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/jeduardo/fic/internal/fic"
	"github.com/spf13/cobra"
)

var compactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Compact a state file by removing checkpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, _ := cmd.Flags().GetString("state")
		out, _ := cmd.Flags().GetString("out")
		progress, _ := cmd.Flags().GetBool("progress")

		if state == "" {
			return fmt.Errorf("--state is required")
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		return fic.CompactStateWithProgress(ctx, state, out, progress)
	},
}

func init() {
	rootCmd.AddCommand(compactCmd)

	compactCmd.Flags().String("state", "", "state file (.fic)")
	compactCmd.Flags().String("out", "", "output file (optional)")
	compactCmd.Flags().Bool("progress", false, "show progress")
}
