package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/natikgadzhi/cli-kit/output"
	"github.com/natikgadzhi/gitbatch/internal/schedule"
	"github.com/spf13/cobra"
)

var (
	scheduleTime    string
	scheduleEvery   string
	scheduleJobs    int
	scheduleDepth   int
	scheduleNoStash bool
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage scheduled sync jobs via macOS LaunchAgent",
}

var scheduleSetCmd = &cobra.Command{
	Use:   "set [directory]",
	Short: "Create or update a scheduled sync",
	Long: `Sets up a macOS LaunchAgent to run "gitbatch sync" on a schedule.
Use --time for daily at a specific time, or --every for an interval.
These flags are mutually exclusive.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if scheduleTime != "" && scheduleEvery != "" {
			return fmt.Errorf("--time and --every are mutually exclusive")
		}
		if scheduleTime == "" && scheduleEvery == "" {
			scheduleTime = "08:00" // default
		}

		dir, err := resolveDir(args)
		if err != nil {
			return err
		}

		binary, err := findBinary()
		if err != nil {
			return err
		}

		// Build sync flags to pass through.
		var syncArgs []string
		if scheduleJobs != 6 {
			syncArgs = append(syncArgs, "-j", strconv.Itoa(scheduleJobs))
		}
		if scheduleDepth != 3 {
			syncArgs = append(syncArgs, "--depth", strconv.Itoa(scheduleDepth))
		}
		if scheduleNoStash {
			syncArgs = append(syncArgs, "--no-stash")
		}

		cfg := schedule.Config{
			Binary:    binary,
			Directory: dir,
			SyncArgs:  syncArgs,
		}

		if scheduleEvery != "" {
			seconds, err := parseInterval(scheduleEvery)
			if err != nil {
				return err
			}
			cfg.IntervalSeconds = seconds
		} else {
			hour, minute, err := parseTime(scheduleTime)
			if err != nil {
				return err
			}
			cfg.Hour = hour
			cfg.Minute = minute
		}

		if err := schedule.Set(cfg); err != nil {
			return err
		}

		if scheduleEvery != "" {
			fmt.Fprintf(os.Stderr, "Schedule set: sync %s every %s\n", dir, scheduleEvery)
		} else {
			fmt.Fprintf(os.Stderr, "Schedule set: sync %s daily at %s\n", dir, scheduleTime)
		}
		fmt.Fprintf(os.Stderr, "Logs: %s\n", schedule.StdoutLogPath())
		return nil
	},
}

var scheduleShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current schedule",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := schedule.Show()
		if err != nil {
			return err
		}

		format := output.Resolve(cmd)
		if output.IsJSON(format) {
			return output.PrintJSON(info)
		}

		loaded := "no"
		if info.Loaded {
			loaded = "yes"
		}

		fmt.Printf("Schedule:  %s\n", info.Schedule)
		fmt.Printf("Directory: %s\n", info.Directory)
		fmt.Printf("Binary:    %s\n", info.Binary)
		if len(info.SyncArgs) > 0 {
			fmt.Printf("Sync args: %s\n", strings.Join(info.SyncArgs, " "))
		}
		fmt.Printf("Loaded:    %s\n", loaded)
		fmt.Printf("Logs:      %s\n", info.StdoutLog)
		return nil
	},
}

var scheduleRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the scheduled sync",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := schedule.Remove(); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Schedule removed.")
		return nil
	},
}

var scheduleRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Trigger the scheduled sync immediately",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := schedule.Run(); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Triggered sync. Check logs with: gitbatch schedule logs")
		return nil
	},
}

var scheduleLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show recent sync logs",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		logPath := schedule.StdoutLogPath()
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			return fmt.Errorf("no logs found at %s — has the schedule run yet?", logPath)
		}

		// Tail the last 50 lines of stdout log.
		tail := exec.Command("tail", "-50", logPath)
		tail.Stdout = os.Stdout
		tail.Stderr = os.Stderr
		return tail.Run()
	},
}

func init() {
	scheduleSetCmd.Flags().StringVar(&scheduleTime, "time", "", "Time to run daily (HH:MM, e.g. 08:00)")
	scheduleSetCmd.Flags().StringVar(&scheduleEvery, "every", "", "Run every interval (e.g. 4h, 30m, 1h30m)")
	scheduleSetCmd.Flags().IntVarP(&scheduleJobs, "jobs", "j", 6, "Max parallel operations for sync")
	scheduleSetCmd.Flags().IntVar(&scheduleDepth, "depth", 3, "Max directory depth for discovery")
	scheduleSetCmd.Flags().BoolVar(&scheduleNoStash, "no-stash", false, "Skip repos with dirty worktrees instead of stashing")

	scheduleCmd.AddCommand(scheduleSetCmd)
	scheduleCmd.AddCommand(scheduleShowCmd)
	scheduleCmd.AddCommand(scheduleRemoveCmd)
	scheduleCmd.AddCommand(scheduleRunCmd)
	scheduleCmd.AddCommand(scheduleLogsCmd)

	rootCmd.AddCommand(scheduleCmd)
}

// parseTime parses "HH:MM" into hour and minute.
func parseTime(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format %q — expected HH:MM", s)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q — must be 0-23", s)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute in %q — must be 0-59", s)
	}
	return hour, minute, nil
}

// parseInterval parses a duration string like "4h", "30m", "1h30m" into seconds.
func parseInterval(s string) (int, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid interval %q — use Go duration format (e.g. 4h, 30m, 1h30m): %w", s, err)
	}
	if d < time.Minute {
		return 0, fmt.Errorf("interval must be at least 1 minute")
	}
	return int(d.Seconds()), nil
}

// findBinary resolves the gitbatch binary path.
func findBinary() (string, error) {
	// Prefer the currently running executable.
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
		if err == nil {
			return exe, nil
		}
	}
	// Fall back to PATH lookup.
	path, err := exec.LookPath("gitbatch")
	if err != nil {
		return "", fmt.Errorf("cannot find gitbatch binary — install it first")
	}
	return path, nil
}
