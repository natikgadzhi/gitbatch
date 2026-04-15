@PROJECT_PROMPT.md

# gitbatch — Development Guide

## Tech Stack

- **Go 1.23+**
- **CLI/Output:** [cli-kit](https://github.com/natikgadzhi/cli-kit) (cobra, table, output, progress, version)
- **Git:** shell out to `git` via `os/exec` (NOT go-git)
- **Parallelism:** `golang.org/x/sync/errgroup`

## Quality Checks

Before committing, every worker MUST verify:

```bash
go build ./...
go vet ./...
go test ./...
```

## Multi-agent Work Environment

### How It Works

1. The lead agent reads `PROJECT_PROMPT.md` and decomposes the project into phased tasks
2. Workers build code in independent worktrees, write tests, commit, and open PRs
3. Reviewers review PRs and keep quality high
4. The lead coordinates, merges PRs, and keeps `main` up to date

### Lead Agent Behavior

1. **Read** `PROJECT_PROMPT.md`
2. **Create tasks** using `TaskCreate` with clear acceptance criteria and dependency chains
3. **Spawn workers** via `Agent` tool with `subagent_type: "general-purpose"`
4. **Track task file state** — move files between `tasks/backlog/`, `tasks/in-progress/`, `tasks/done/`
5. **Assign reviews** once a worker opens a PR
6. **Shut down** when all tasks are complete

### Worker Agent Instructions

1. **Read the task** with `TaskGet <task-id>`
2. **Mark in-progress** with `TaskUpdate`
3. **Create a git worktree**:
   ```bash
   git fetch origin && git pull --ff
   git worktree add ../worktrees/gitbatch-task-N -b task-N-description
   cd ../worktrees/gitbatch-task-N
   ```
4. **Read existing code** before writing
5. **Implement the task** with tests
6. **Verify**: `go build ./...`, `go vet ./...`, `go test ./...`
7. **Commit**: `[task-N] <description>`
8. **Push and create PR**
9. **Clean up worktree** after merge
10. **Mark task completed**

## Git Conventions

- Main checkout stays on `main` — workers use worktrees
- Every code change goes through a PR
- Commit format: `[task-N] <description>`

## Task File System

```
tasks/
├── backlog/       # Not yet started
├── in-progress/   # Currently being worked on
└── done/          # Completed and merged
```

## Releases

```sh
gh workflow run release.yml --field version_bump=patch
```

Uses GoReleaser. Single workflow for tag + release.

## Important Rules

- **Never modify PROJECT_PROMPT.md**
- **Always read before writing**
- **Test everything**
- **No premature abstraction**
- **Keep README.md in sync** with CLI surface
- **Use cli-kit** for table output, progress, version command, output format detection
