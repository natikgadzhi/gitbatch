# Task 3: Git operations

## Objective
Implement all git operations needed for the pull workflow, shelling out to `git` via `os/exec`.

## Depends on
- Task 1 (scaffold — need the module and Repo struct)

## Acceptance criteria
- `internal/git/operations.go` with these functions:
  - `DetectRemote(repoPath string) (string, error)` — prefer `origin`, fall back to `upstream`, error if neither
  - `CurrentBranch(repoPath string) (string, error)` — `git symbolic-ref --short HEAD`, error on detached
  - `IsDirty(repoPath string) (bool, error)` — `git status --porcelain`, dirty if output non-empty
  - `Fetch(repoPath, remote, branch string) error` — `git fetch <remote> <branch>`
  - `MergeFF(repoPath, remote, branch string) (bool, error)` — `git merge --ff-only <remote>/<branch>`, returns true if updated (not "already up to date")
  - `StashPush(repoPath string) error` — `git stash push -m "gitbatch auto-stash"`
  - `StashPop(repoPath string) (conflict bool, err error)` — `git stash pop`, detect conflicts from exit code/output
- All functions take `repoPath string` and use `git -C <path>` to target the repo
- Helper: `func gitCmd(repoPath string, args ...string) (string, error)` that runs `git -C <repoPath> <args>`, captures combined stdout+stderr, returns output and error
- `internal/git/operations_test.go`:
  - Test helper that creates a real git repo in a temp dir with a commit
  - Test helper that creates a repo with a remote (use `git clone --bare` + `git remote add`)
  - Test: DetectRemote finds origin
  - Test: DetectRemote falls back to upstream
  - Test: DetectRemote errors when no remote
  - Test: CurrentBranch returns branch name
  - Test: IsDirty detects clean and dirty states
  - Test: Fetch succeeds with valid remote
  - Test: MergeFF fast-forwards and returns true
  - Test: MergeFF returns false when already up to date
  - Test: StashPush and StashPop round-trip
- `go test ./internal/git/...` passes

## Notes
- Keep error messages descriptive — include the repo path and git output on failure
- Don't swallow stderr — include it in error messages
