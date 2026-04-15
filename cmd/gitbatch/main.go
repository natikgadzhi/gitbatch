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
	Use:   "gitbatch [flags] [directory]",
	Short: "Batch-update multiple git repositories in parallel",
	Long: `gitbatch discovers git repositories under a root directory and
fetches/fast-forward merges them all in parallel. For repos with dirty
worktrees it can stash changes, pull, and reapply the stash.`,
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Resolve directory.
		dir, err := resolveDir(args)
		if err != nil {
			return err
		}

		// 2. Discover repositories.
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

		// 3. Set up progress indicator.
		counter := progress.NewCounter(
			fmt.Sprintf("Updating %d repos", len(repos)),
			format,
		)

		// 4. Run the parallel updater.
		results := runner.Run(cmd.Context(), repos, jobs, noStash, func(done int) {
			counter.Update(done)
		})

		counter.Finish()

		// 5. Output results.
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

		// 6. Exit code: 1 if any failed or conflicted.
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

	// Custom flags.
	rootCmd.Flags().IntVarP(&jobs, "jobs", "j", 6, "Max parallel operations")
	rootCmd.Flags().IntVar(&depth, "depth", 3, "Max directory depth for discovery")
	rootCmd.Flags().BoolVar(&noStash, "no-stash", false, "Skip repos with dirty worktrees instead of stashing")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
