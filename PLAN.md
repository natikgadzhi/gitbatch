# Plan: gitbatch

Simple CLI that discovers git repos and pulls them all in parallel. Uses cli-kit for output.

## Architecture

```
cmd/gitbatch/
  main.go              # Entry point, cobra root command via cli-kit

internal/
  git/
    discovery.go       # Walk directories, find .git repos
    repo.go            # Repo struct, status enum
    operations.go      # Fetch, pull, stash — all via os/exec
    operations_test.go
    discovery_test.go
  runner/
    runner.go          # Parallel executor with errgroup
    runner_test.go
```

No TUI. No bubbletea. Just discover, pull in parallel, print a table.

## Data flow

```
main.go
  → git.Discover(rootDir, depth)           → []Repo
  → runner.Run(ctx, repos, concurrency)    → []Result
  → cli-kit/table to print results
  → exit code based on results
```

## Tasks

### Task 1: Scaffold Go project
- `go mod init`, add cli-kit + x/sync deps
- Directory structure, Makefile, goreleaser, release workflow
- Minimal main.go with cobra via cli-kit (version command works)
- Acceptance: `go build ./...` passes, `gitbatch --version` works

### Task 2: Repository discovery
- `internal/git/discovery.go` + tests
- Walk dirs, find `.git`, skip `.build`/`node_modules`/`vendor`
- Respect depth limit
- Acceptance: tests pass with temp dir fixtures

### Task 3: Git operations
- `internal/git/operations.go` + tests
- DetectRemote, CurrentBranch, IsDirty, Fetch, MergeFF, StashPush, StashPop
- All via os/exec
- Acceptance: tests pass with real temp git repos

### Task 4: Parallel runner + CLI integration
- `internal/runner/runner.go` — errgroup with semaphore
- Wire into main.go: discover → run → print table → exit code
- All flags: -j, --depth, --no-stash, -o
- Acceptance: end-to-end works, table prints, exit codes correct

## Execution order

```
Task 1 (scaffold)
  ├── Task 2 (discovery)    — parallel
  └── Task 3 (git ops)      — parallel
       └── Task 4 (runner + integration) — after 2+3
```
