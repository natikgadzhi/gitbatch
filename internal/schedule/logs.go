package schedule

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/natikgadzhi/gitbatch/internal/runner"
)

// LastRun describes the outcome of the most recent scheduled sync as
// reconstructed from the stdout log. FinishedAt is the log file's mtime
// — close enough to "when the run ended" for display purposes, and lets
// us work with any historical log format that emits a JSON array at the
// end of each run.
type LastRun struct {
	FinishedAt time.Time       `json:"finished_at"`
	Results    []runner.Result `json:"results"`
}

// ReadLastRun returns the most recent run from the stdout log, or
// (nil, nil) when the log is missing or contains no decodable results.
func ReadLastRun() (*LastRun, error) {
	path := StdoutLogPath()
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat log: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening log: %w", err)
	}
	defer f.Close()

	results, err := decodeLastResults(f)
	if err != nil || results == nil {
		return nil, err
	}

	return &LastRun{FinishedAt: stat.ModTime(), Results: results}, nil
}

// decodeLastResults streams JSON values from r (each sync run appends one
// array) and returns the last one successfully decoded.
func decodeLastResults(r io.Reader) ([]runner.Result, error) {
	dec := json.NewDecoder(r)
	var last []runner.Result
	for {
		var results []runner.Result
		err := dec.Decode(&results)
		if err == io.EOF {
			break
		}
		if err != nil {
			// Tolerate a malformed tail (partial write, stray text).
			break
		}
		last = results
	}
	return last, nil
}

// ExpectedFireBefore computes the most recent scheduled fire time at or
// before t for a daily Hour:Minute schedule. Returns zero time if the
// schedule isn't daily.
func ExpectedFireBefore(info *Info, t time.Time) time.Time {
	if info == nil || info.Hour == nil || info.Minute == nil {
		return time.Time{}
	}
	loc := t.Location()
	candidate := time.Date(t.Year(), t.Month(), t.Day(), *info.Hour, *info.Minute, 0, 0, loc)
	if candidate.After(t) {
		candidate = candidate.AddDate(0, 0, -1)
	}
	return candidate
}
