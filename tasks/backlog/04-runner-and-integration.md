# Task 4: Parallel runner and CLI integration

## Objective
Build the parallel runner that processes repos concurrently, wire everything into main.go, and produce the final working CLI.

## Depends on
- Task 2 (discovery)
- Task 3 (git operations)

## Acceptance criteria

### Runner (`internal/runner/runner.go`)
- `Result` struct: `Repo git.Repo`, `Status string`, `Detail string`
- Status constants: `StatusOK`, `StatusUpdated`, `StatusStashed`, `StatusSkipped`, `StatusFailed`, `StatusConflict`
- `Run(ctx context.Context, repos []git.Repo, concurrency int, noStash bool) []Result`
  - Uses `errgroup` with `SetLimit(concurrency)` as semaphore
  - Per-repo logic:
    1. DetectRemote → skip if error
    2. CurrentBranch → skip if detached
    3. IsDirty → if dirty and noStash: skip. If dirty and !noStash: StashPush
    4. Fetch
    5. MergeFF
    6. If stashed: StashPop → detect conflict
    7. Return appropriate status
  - Collects results (thread-safe via mutex or pre-allocated slice indexed by position)
  - Returns results in same order as input repos

### CLI integration (`cmd/gitbatch/main.go`)
- Root command RunE:
  1. Resolve directory (positional arg or cwd)
  2. `git.Discover(dir, depth)`
  3. Print "Found N repositories in <dir>" to stderr
  4. `runner.Run(ctx, repos, jobs, noStash)`
  5. Print results table via cli-kit/table with columns: REPO, BRANCH, STATUS, DETAIL
  6. Exit 0 if all OK/UPDATED/STASHED/SKIPPED, exit 1 if any FAILED/CONFLICT
- `-o json` outputs JSON array of results
- Progress: use cli-kit/progress counter on stderr showing "Updating repos... N/total"

### Runner tests (`internal/runner/runner_test.go`)
- Test: processes repos and returns correct statuses
- Test: respects concurrency limit (can verify via timing or channel observation)
- Test: noStash flag skips dirty repos

## Notes
- For the progress counter, runner needs a callback or channel. Simplest: pass a `func(done int)` callback that the CLI wires to cli-kit/progress.
- The result slice can be pre-allocated to len(repos) and indexed by position — no mutex needed.
- Keep the CLI simple — this is not interactive. Print table, done.
