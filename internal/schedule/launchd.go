package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	label    = "com.natikgadzhi.gitbatch"
	plistFmt = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>sync</string>
        <string>-o</string>
        <string>json</string>
        <string>%s</string>
    </array>
%s
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
    <key>RunAtLoad</key>
    <false/>
</dict>
</plist>
`
)

// Config holds the schedule configuration.
type Config struct {
	Binary    string
	Directory string
	Hour      int
	Minute    int
	// IntervalSeconds is used for --every; if > 0, it takes precedence over Hour/Minute.
	IntervalSeconds int
}

// Info holds parsed schedule information for display.
type Info struct {
	Label     string `json:"label"`
	Binary    string `json:"binary"`
	Directory string `json:"directory"`
	Schedule  string `json:"schedule"`
	StdoutLog string `json:"stdout_log"`
	StderrLog string `json:"stderr_log"`
	Loaded    bool   `json:"loaded"`
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

	content := fmt.Sprintf(plistFmt,
		label,
		cfg.Binary,
		cfg.Directory,
		scheduleBlock(cfg),
		StdoutLogPath(),
		StderrLogPath(),
	)

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
	info := &Info{
		Label:     label,
		Binary:    extractPlistString(content, "ProgramArguments"),
		Directory: extractDirectory(content),
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

// extractDirectory pulls the directory argument from the plist (last string in ProgramArguments).
func extractDirectory(content string) string {
	// The directory is the last <string> in ProgramArguments array.
	inArray := false
	last := ""
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
			last = trimmed[len("<string>") : len(trimmed)-len("</string>")]
		}
	}
	return last
}

// extractPlistString pulls the first <string> value after ProgramArguments (the binary path).
func extractPlistString(content, _ string) string {
	inArray := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "<key>ProgramArguments</key>" {
			inArray = true
			continue
		}
		if inArray && strings.HasPrefix(trimmed, "<string>") && strings.HasSuffix(trimmed, "</string>") {
			return trimmed[len("<string>") : len(trimmed)-len("</string>")]
		}
	}
	return ""
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
		return fmt.Sprintf("daily at %02d:%02d", hour, minute)
	}

	return "unknown"
}
