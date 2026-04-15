package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/cli-kit/progress"
	"github.com/natikgadzhi/cli-kit/table"
	"github.com/natikgadzhi/cli-kit/version"
	"github.com/natikgadzhi/gitbatch/internal/git"
	"github.com/natikgadzhi/gitbatch/internal/runner"
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
	Use:   "gitbatch",
	Short: "Batch-update git repositories in parallel",
	Long: `gitbatch discovers git repositories under a directory and pulls them
all in parallel. It handles dirty worktrees by stashing, pulling, and
reapplying changes automatically.

Examples:
  gitbatch sync ~/src/myorg
  gitbatch sync -j 12 --depth 5 ~/projects
  gitbatch schedule set --time 8am ~/src/myorg`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

var syncCmd = &cobra.Command{
	Use:   "sync [directory]",
	Short: "Fetch and fast-forward merge all repositories in parallel",
	Long: `Discovers git repositories under the given directory (default: current
directory) and fetches/fast-forward merges them all in parallel.

For repos with uncommitted changes, gitbatch stashes them before pulling
and reapplies them after. Use --no-stash to skip dirty repos instead.

Examples:
  gitbatch sync ~/src
  gitbatch sync -j 12 --depth 5 ~/projects
  gitbatch sync --no-stash ~/src`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := resolveDir(args)
		if err != nil {
			return err
		}

		repos, err := git.Discover(dir, depth)
		if err != nil {
			return err
		}

		format := output.Resolve(cmd)

		if len(repos) == 0 {
			fmt.Fprintf(os.Stderr, "No repositories found in %s\n", dir)
			return nil
		}

		noun := "repositories"
		if len(repos) == 1 {
			noun = "repository"
		}
		fmt.Fprintf(os.Stderr, "Found %d %s in %s\n", len(repos), noun, dir)

		counter := progress.NewCounter(
			fmt.Sprintf("Updating %d repos", len(repos)),
			format,
		)

		results := runner.Run(cmd.Context(), repos, jobs, noStash, func(done int) {
			counter.Update(done)
		})

		counter.Finish()

		if output.IsJSON(format) {
			if err := output.PrintJSON(results); err != nil {
				return fmt.Errorf("writing JSON output: %w", err)
			}
		} else {
			t := table.New()
			t.Header("Repo", "Branch", "Status", "Detail")
			for _, r := range results {
				t.Row(r.Repo.RelPath, r.Branch, r.Status, r.Detail)
			}
			if err := t.Flush(); err != nil {
				return fmt.Errorf("writing table output: %w", err)
			}
		}

		for _, r := range results {
			if r.Status == runner.StatusFailed || r.Status == runner.StatusConflict {
				os.Exit(1)
			}
		}

		return nil
	},
}

func resolveDir(args []string) (string, error) {
	if len(args) > 0 {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return "", fmt.Errorf("resolving directory: %w", err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("accessing directory: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("not a directory: %s", abs)
		}
		return abs, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	return dir, nil
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

	// Sync command flags.
	syncCmd.Flags().IntVarP(&jobs, "jobs", "j", 6, "Max parallel operations")
	syncCmd.Flags().IntVar(&depth, "depth", 3, "Max directory depth for discovery")
	syncCmd.Flags().BoolVar(&noStash, "no-stash", false, "Skip repos with dirty worktrees instead of stashing")

	rootCmd.AddCommand(syncCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
