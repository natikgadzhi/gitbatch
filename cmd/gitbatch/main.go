package main

import (
	"fmt"
	"os"

	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/version"
	"github.com/spf13/cobra"
)

// Version, Commit, and Date are set at build time via ldflags.
var (
	Version = "dev"
	Commit  = "dev"
	Date    = "unknown"
)

var (
	jobs    int
	depth   int
	noStash bool
)

var rootCmd = &cobra.Command{
	Use:   "gitbatch [flags] [directory]",
	Short: "Batch-update multiple git repositories in parallel",
	Long: `gitbatch discovers git repositories under a root directory and
fetches/fast-forward merges them all in parallel. For repos with dirty
worktrees it can stash changes, pull, and reapply the stash.`,
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	// Register cli-kit output flag (-o/--output).
	output.RegisterFlag(rootCmd)

	// Register cli-kit version command and --version flag.
	info := &version.Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
	}
	rootCmd.AddCommand(version.NewCommand(info))
	version.SetupFlag(rootCmd, info)

	// Custom flags.
	rootCmd.Flags().IntVarP(&jobs, "jobs", "j", 6, "Max parallel operations")
	rootCmd.Flags().IntVar(&depth, "depth", 3, "Max directory depth for discovery")
	rootCmd.Flags().BoolVar(&noStash, "no-stash", false, "Skip repos with dirty worktrees instead of stashing")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
