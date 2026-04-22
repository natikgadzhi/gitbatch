# Task 5: `schedule show` displays last run

## Objective
Enhance `gitbatch schedule show` to display information about the last
scheduled run: when it was expected, when it actually started/finished,
and a table of per-repo results.

## Motivation
Users set up a scheduled sync and want a single command to verify it's
healthy. Today `schedule show` only confirms that a plist exists and is
loaded; `schedule logs` tails raw output. The user needs a structured
"last run summary" view that answers: *did it run when expected, and did
every repo sync OK?*

## Acceptance criteria

### Sync JSON output (`cmd/gitbatch/main.go`, `internal/runner`)
- `gitbatch sync -o json` emits a single JSON object envelope:
  ```json
  {
    "started_at": "...",
    "finished_at": "...",
    "directory": "...",
    "results": [ ... ]
  }
  ```
- Non-JSON output unchanged.
- `runner.RunSummary` type added.

### Schedule info (`internal/schedule/launchd.go`)
- `Info` gains `Hour`, `Minute`, `IntervalSeconds` (pointers; nil when n/a)
  so callers can reason about expected fire times.

### Last-run parsing (`internal/schedule/logs.go`, new)
- `LastRun() (*runner.RunSummary, error)` reads stdout.log, streams JSON
  objects, returns the last successfully decoded one. Returns a nil
  summary with no error if the log doesn't exist or has no complete run.

### Show command (`cmd/gitbatch/schedule.go`)
- `scheduleShowCmd` prints current schedule as before, then a blank line,
  then:
  - "Last run" section with scheduled fire (daily only), started,
    finished, duration.
  - Table of per-repo results (cli-kit/table).
- JSON output includes a `last_run` key on the Info payload.
- If no run yet, prints "No runs yet." instead of the table.

### Tests
- `internal/schedule/logs_test.go` covers: missing log, multiple
  concatenated runs, malformed tail.

## Notes
- The wrapping of sync JSON output is a deliberate breaking change to the
  JSON format. The project is pre-1.0 and the wrapper is strictly more
  useful for downstream consumers.
