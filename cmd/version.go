package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var commit = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print build version",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "FIC (Filesystem Integrity Checker) - version %s (https://github.com/jeduardo/fic/)\n", commit)
		return err
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
