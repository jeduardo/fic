package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "fic",
	Short: "Filesystem Integrity Checker",
}

var exitFunc = os.Exit

func Execute() {
	rootCmd.SilenceUsage = true
	if err := rootCmd.Execute(); err != nil {
		exitFunc(1)
	}
}

// SetExitFuncForTest overrides the exit function and returns a restore closure.
func SetExitFuncForTest(fn func(int)) func() {
	old := exitFunc
	exitFunc = fn
	return func() {
		exitFunc = old
	}
}
