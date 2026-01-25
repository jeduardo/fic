package cmd

import (
	"fmt"

	"github.com/jeduardo/fic/internal/fic"
	"github.com/spf13/cobra"
)

var compareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare two state files",
	RunE: func(cmd *cobra.Command, args []string) error {
		left, _ := cmd.Flags().GetString("left")
		right, _ := cmd.Flags().GetString("right")
		format, _ := cmd.Flags().GetString("format")
		out, _ := cmd.Flags().GetString("out")

		if left == "" || right == "" {
			return fmt.Errorf("--left and --right are required")
		}

		return fic.RunCompare(left, right, format, out)
	},
}

func init() {
	rootCmd.AddCommand(compareCmd)

	compareCmd.Flags().String("left", "", "left state file (.fic)")
	compareCmd.Flags().String("right", "", "right state file (.fic)")
	compareCmd.Flags().String("format", "text", "output format: text or json")
	compareCmd.Flags().String("out", "", "output file (optional)")
}
