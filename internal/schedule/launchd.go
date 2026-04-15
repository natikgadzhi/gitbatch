package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const label = "com.natikgadzhi.gitbatch"

// Config holds the schedule configuration.
type Config struct {
	Binary    string
	Directory string
	Hour      int
	Minute    int
	// IntervalSeconds is used for --every; if > 0, it takes precedence over Hour/Minute.
	IntervalSeconds int
	// SyncArgs are additional flags passed to "gitbatch sync" (e.g. "-j", "12", "--depth", "5").
	SyncArgs []string
}

func buildPlist(cfg Config) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>`)
	b.WriteString(label)
	b.WriteString(`</string>
    <key>ProgramArguments</key>
    <array>
        <string>`)
	b.WriteString(cfg.Binary)
	b.WriteString(`</string>
        <string>sync</string>
        <string>-o</string>
        <string>json</string>`)
	for _, arg := range cfg.SyncArgs {
		b.WriteString("\n        <string>")
		b.WriteString(arg)
		b.WriteString("</string>")
	}
	b.WriteString("\n        <string>")
	b.WriteString(cfg.Directory)
	b.WriteString(`</string>
    </array>
`)
	b.WriteString(scheduleBlock(cfg))
	b.WriteString(`
    <key>StandardOutPath</key>
    <string>`)
	b.WriteString(StdoutLogPath())
	b.WriteString(`</string>
    <key>StandardErrorPath</key>
    <string>`)
	b.WriteString(StderrLogPath())
	b.WriteString(`</string>
    <key>RunAtLoad</key>
    <false/>
</dict>
</plist>
`)
	return b.String()
}

// Info holds parsed schedule information for display.
type Info struct {
	Label     string   `json:"label"`
	Binary    string   `json:"binary"`
	Directory string   `json:"directory"`
	SyncArgs  []string `json:"sync_args,omitempty"`
	Schedule  string   `json:"schedule"`
	StdoutLog string   `json:"stdout_log"`
	StderrLog string   `json:"stderr_log"`
	Loaded    bool     `json:"loaded"`
}

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func logDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "gitbatch", "logs")
}

// StdoutLogPath returns the path to the stdout log file.
func StdoutLogPath() string {
	return filepath.Join(logDir(), "stdout.log")
}

// StderrLogPath returns the path to the stderr log file.
func StderrLogPath() string {
	return filepath.Join(logDir(), "stderr.log")
}

func scheduleBlock(cfg Config) string {
	if cfg.IntervalSeconds > 0 {
		return fmt.Sprintf("    <key>StartInterval</key>\n    <integer>%d</integer>", cfg.IntervalSeconds)
	}
	return fmt.Sprintf(`    <key>StartCalendarInterval</key>
    <dict>
        <key>Hour</key>
        <integer>%d</integer>
        <key>Minute</key>
        <integer>%d</integer>
    </dict>`, cfg.Hour, cfg.Minute)
}

// Set creates and loads the LaunchAgent plist.
func Set(cfg Config) error {
	// Unload existing if present.
	if _, err := os.Stat(plistPath()); err == nil {
		_ = unload()
	}

	if err := os.MkdirAll(filepath.Dir(plistPath()), 0o755); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}
	if err := os.MkdirAll(logDir(), 0o755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	content := buildPlist(cfg)

	if err := os.WriteFile(plistPath(), []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	return load()
}

// Remove unloads and deletes the LaunchAgent plist.
func Remove() error {
	if _, err := os.Stat(plistPath()); os.IsNotExist(err) {
		return fmt.Errorf("no schedule found (no plist at %s)", plistPath())
	}

	_ = unload()

	if err := os.Remove(plistPath()); err != nil {
		return fmt.Errorf("removing plist: %w", err)
	}
	return nil
}

// Run triggers the LaunchAgent immediately.
func Run() error {
	if !isLoaded() {
		return fmt.Errorf("schedule is not loaded — run `gitbatch schedule set` first")
	}
	cmd := exec.Command("launchctl", "start", label)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl start: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Show returns information about the current schedule.
func Show() (*Info, error) {
	data, err := os.ReadFile(plistPath())
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("no schedule configured (no plist at %s)", plistPath())
	}
	if err != nil {
		return nil, fmt.Errorf("reading plist: %w", err)
	}

	content := string(data)
	progArgs := extractProgramArguments(content)
	info := &Info{
		Label:     label,
		Binary:    progArgs.binary,
		Directory: progArgs.directory,
		SyncArgs:  progArgs.syncArgs,
		Schedule:  extractSchedule(content),
		StdoutLog: StdoutLogPath(),
		StderrLog: StderrLogPath(),
		Loaded:    isLoaded(),
	}

	return info, nil
}

func load() error {
	cmd := exec.Command("launchctl", "load", plistPath())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func unload() error {
	cmd := exec.Command("launchctl", "unload", plistPath())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl unload: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func isLoaded() bool {
	cmd := exec.Command("launchctl", "list", label)
	return cmd.Run() == nil
}

type programArgs struct {
	binary    string
	directory string
	syncArgs  []string
}

// extractProgramArguments parses the ProgramArguments array from the plist.
// Expected order: binary, "sync", "-o", "json", [syncArgs...], directory
func extractProgramArguments(content string) programArgs {
	inArray := false
	var args []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "<key>ProgramArguments</key>" {
			inArray = true
			continue
		}
		if inArray && trimmed == "</array>" {
			break
		}
		if inArray && strings.HasPrefix(trimmed, "<string>") && strings.HasSuffix(trimmed, "</string>") {
			args = append(args, trimmed[len("<string>"):len(trimmed)-len("</string>")])
		}
	}

	var result programArgs
	if len(args) == 0 {
		return result
	}
	result.binary = args[0]
	// Last arg is always the directory.
	if len(args) > 1 {
		result.directory = args[len(args)-1]
	}
	// Args between the fixed prefix ("binary", "sync", "-o", "json") and the directory are sync args.
	if len(args) > 5 {
		result.syncArgs = args[4 : len(args)-1]
	}
	return result
}

// extractSchedule returns a human-readable schedule description.
func extractSchedule(content string) string {
	// Check for StartInterval first.
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "<key>StartInterval</key>" {
			lines := strings.Split(content, "\n")
			if i+1 < len(lines) {
				val := strings.TrimSpace(lines[i+1])
				val = strings.TrimPrefix(val, "<integer>")
				val = strings.TrimSuffix(val, "</integer>")
				seconds := 0
				fmt.Sscanf(val, "%d", &seconds)
				d := time.Duration(seconds) * time.Second
				return fmt.Sprintf("every %s", d)
			}
		}
	}

	// Check for StartCalendarInterval.
	hour, minute := -1, -1
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "<key>Hour</key>" && i+1 < len(lines) {
			val := strings.TrimSpace(lines[i+1])
			val = strings.TrimPrefix(val, "<integer>")
			val = strings.TrimSuffix(val, "</integer>")
			fmt.Sscanf(val, "%d", &hour)
		}
		if trimmed == "<key>Minute</key>" && i+1 < len(lines) {
			val := strings.TrimSpace(lines[i+1])
			val = strings.TrimPrefix(val, "<integer>")
			val = strings.TrimSuffix(val, "</integer>")
			fmt.Sscanf(val, "%d", &minute)
		}
	}
	if hour >= 0 && minute >= 0 {
		return fmt.Sprintf("daily at %s", formatTime12h(hour, minute))
	}

	return "unknown"
}

func formatTime12h(hour, minute int) string {
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
