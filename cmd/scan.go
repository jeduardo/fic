package cmd

import (
	"fmt"
	"runtime"

	"github.com/jeduardo/fic/internal/fic"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a directory and write a state file",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := cmd.Flags().GetString("root")
		out, _ := cmd.Flags().GetString("out")
		algo, _ := cmd.Flags().GetString("algo")
		workers, _ := cmd.Flags().GetInt("workers")
		progress, _ := cmd.Flags().GetBool("progress")
		followSymlinks, _ := cmd.Flags().GetBool("follow-symlinks")

		if root == "" || out == "" {
			return fmt.Errorf("--root and --out are required")
		}

		opts := fic.ScanOptions{
			Root:           root,
			StatePath:      out,
			Algo:           algo,
			Workers:        workers,
			Progress:       progress,
			FollowSymlinks: followSymlinks,
		}

		return fic.RunScan(opts)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().String("root", "", "root directory to scan")
	scanCmd.Flags().String("out", "", "output state file (.fic)")
	scanCmd.Flags().String("algo", "sha256", "hash algorithm (sha256, md5)")
	scanCmd.Flags().Int("workers", runtime.NumCPU(), "number of parallel workers")
	scanCmd.Flags().Bool("progress", false, "show progress")
	scanCmd.Flags().Bool("follow-symlinks", false, "follow symlinks")
}
