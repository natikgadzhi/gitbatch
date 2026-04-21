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
	"github.com/natikgadzhi/cli-kit/table"
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
	Short: "Set up automatic sync on a schedule (macOS)",
	Long: `Manage a macOS LaunchAgent that runs "gitbatch sync" automatically.

The schedule survives reboots. If your laptop is asleep at the scheduled
time, the sync runs automatically when you open the lid.

Examples:
  gitbatch schedule set --time 8am ~/src
  gitbatch schedule set --every 4h ~/src
  gitbatch schedule show
  gitbatch schedule logs`,
}

var scheduleSetCmd = &cobra.Command{
	Use:   "set [directory]",
	Short: "Create or update the sync schedule",
	Long: `Sets up a macOS LaunchAgent to run "gitbatch sync" automatically.

Use --time for a daily schedule or --every for a recurring interval.
These are mutually exclusive; if neither is given, defaults to --time 8am.

Sync flags (--jobs, --depth, --no-stash) are saved into the schedule
and used on every run.

Examples:
  gitbatch schedule set --time 8am ~/src
  gitbatch schedule set --time 2:30pm -j 12 ~/projects
  gitbatch schedule set --every 4h --no-stash ~/src`,
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
			fmt.Fprintf(os.Stderr, "Schedule set: sync %s daily at %s\n", dir, formatTime(cfg.Hour, cfg.Minute))
		}
		fmt.Fprintf(os.Stderr, "Logs: %s\n", schedule.StdoutLogPath())
		return nil
	},
}

var scheduleShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current schedule, directory, status, and last run",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := schedule.Show()
		if err != nil {
			return err
		}

		lastRun, err := schedule.ReadLastRun()
		if err != nil {
			return err
		}

		format := output.Resolve(cmd)
		if output.IsJSON(format) {
			payload := struct {
				*schedule.Info
				LastRun *schedule.LastRun `json:"last_run,omitempty"`
			}{Info: info, LastRun: lastRun}
			return output.PrintJSON(payload)
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

		fmt.Println()
		fmt.Println("Last run:")
		if lastRun == nil {
			fmt.Println("  No runs yet.")
			return nil
		}

		finished := lastRun.FinishedAt.Local()
		if expected := schedule.ExpectedFireBefore(info, finished); !expected.IsZero() {
			delay := finished.Sub(expected).Round(time.Second)
			fmt.Printf("  Scheduled: %s\n", expected.Format("2006-01-02 3:04pm"))
			fmt.Printf("  Finished:  %s (%s after scheduled)\n", finished.Format("2006-01-02 3:04:05pm"), delay)
		} else {
			fmt.Printf("  Finished:  %s\n", finished.Format("2006-01-02 3:04:05pm"))
		}
		fmt.Println()

		if len(lastRun.Results) == 0 {
			fmt.Println("  (log contains no per-repo results)")
			return nil
		}
		t := table.New()
		t.Header("Repo", "Status", "Detail")
		t.WrapColumns(2)
		for _, r := range lastRun.Results {
			t.Row(r.Repo.RelPath, r.Status, r.Detail)
		}
		if err := t.Flush(); err != nil {
			return fmt.Errorf("writing table output: %w", err)
		}
		return nil
	},
}

var scheduleRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Unload and delete the scheduled sync",
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
	Short: "Trigger the scheduled sync now (runs in background via launchd)",
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
	Short: "Tail the last 50 lines of scheduled sync output",
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
	scheduleSetCmd.Flags().StringVar(&scheduleTime, "time", "", "Time to run daily (e.g. 8am, 2:30pm, 14:30)")
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

// parseTime parses time strings in various formats:
//
//	"8am", "8pm", "8:30am", "2:30pm", "08:00", "14:30"
func parseTime(s string) (int, int, error) {
	lower := strings.ToLower(strings.TrimSpace(s))

	// Try 12-hour formats: 8am, 8pm, 8:30am, 2:30pm
	for _, suffix := range []string{"am", "pm"} {
		if strings.HasSuffix(lower, suffix) {
			body := strings.TrimSuffix(lower, suffix)
			hour, minute, err := parseHourMinute(body)
			if err != nil {
				return 0, 0, fmt.Errorf("invalid time %q: %w", s, err)
			}
			if hour < 1 || hour > 12 {
				return 0, 0, fmt.Errorf("invalid hour in %q — must be 1-12 with am/pm", s)
			}
			if suffix == "am" && hour == 12 {
				hour = 0
			} else if suffix == "pm" && hour != 12 {
				hour += 12
			}
			return hour, minute, nil
		}
	}

	// 24-hour format: 08:00, 14:30
	hour, minute, err := parseHourMinute(lower)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time %q — expected HH:MM, 8am, 2:30pm, etc.", s)
	}
	if hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q — must be 0-23", s)
	}
	return hour, minute, nil
}

func formatTime(hour, minute int) string {
	suffix := "am"
	h := hour
	if h == 0 {
		h = 12
	} else if h == 12 {
		suffix = "pm"
	} else if h > 12 {
		h -= 12
		suffix = "pm"
	}
	if minute == 0 {
		return fmt.Sprintf("%d%s", h, suffix)
	}
	return fmt.Sprintf("%d:%02d%s", h, minute, suffix)
}

func parseHourMinute(s string) (int, int, error) {
	if strings.Contains(s, ":") {
		parts := strings.SplitN(s, ":", 2)
		hour, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid hour")
		}
		minute, err := strconv.Atoi(parts[1])
		if err != nil || minute < 0 || minute > 59 {
			return 0, 0, fmt.Errorf("invalid minute — must be 0-59")
		}
		return hour, minute, nil
	}
	// Just an hour, no minutes: "8", "14"
	hour, err := strconv.Atoi(s)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	return hour, 0, nil
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
