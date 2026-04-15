# gitbatch

A simple CLI tool that batch-updates multiple git repositories in parallel.

## What it does

Given a root directory (default: `~/src/natikgadzhi`), gitbatch:

1. Discovers all git repositories recursively
2. Fetches and fast-forward merges all repos in parallel
3. For repos with dirty worktrees: stashes changes, pulls, then reapplies the stash
4. Prints a summary table of results

Designed to run interactively or on a cron schedule (e.g. every morning).

## Tech stack

- **Language:** Go 1.23+
- **CLI/Output:** [cli-kit](https://github.com/natikgadzhi/cli-kit) (cobra, table, progress, version)
- **Git operations:** Shell out to `git` via `os/exec` — full compatibility with user's git config, SSH keys, credential helpers
- **Parallelism:** `golang.org/x/sync/errgroup`

## Core behavior

### Repository discovery

- Walk the directory tree starting from a configurable root
- A directory is a git repo if it contains a `.git` directory or file (worktrees use a file)
- Skip directories named `.build`, `node_modules`, `.git`, `vendor`
- Depth limit: configurable, default 3 levels deep

### Pull strategy

For each repo, in parallel:

1. Determine the remote: prefer `origin`, fall back to `upstream`, skip if neither exists
2. Get the current branch via `git symbolic-ref --short HEAD`. Skip if detached HEAD.
3. Check if the worktree is dirty (`git status --porcelain`)
4. If dirty: `git stash push -m "gitbatch auto-stash"`
5. `git fetch <remote> <branch>`
6. `git merge --ff-only <remote>/<branch>`
7. If stashed: `git stash pop`
8. If stash pop conflicts: leave the stash and report the conflict (don't lose work)

### Output

Uses cli-kit table when TTY, plain/json when piped:

```
 REPO              BRANCH      STATUS    DETAIL
 fm                main        UPDATED   fast-forwarded origin/main
 gdrive-cli        main        OK        already up to date
 slack-cli         feat/auth   STASHED   stashed -> pulled -> reapplied
 scripts           main        CONFLICT  stash pop conflict — stash preserved
 template          main        SKIPPED   no remote
```

When running on cron (no TTY), outputs JSON for easy log parsing.

### CLI interface

```
gitbatch [flags] [directory]

Flags:
  -j, --jobs <n>       Max parallel operations (default: 6)
      --depth <n>      Max directory depth for discovery (default: 3)
      --no-stash       Skip repos with dirty worktrees instead of stashing
  -o, --output         Output format: table, json (default: auto via TTY)
      --version        Print version
  -h, --help           Show help
```

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | All repos updated successfully (or already up to date) |
| 1 | Some repos failed (partial success) |
| 2 | Fatal error (bad arguments, root dir doesn't exist) |

## Non-goals

- Interactive TUI / live-updating display
- Cloning new repos
- Branch switching
- Push operations
- Config file (flags are enough)
