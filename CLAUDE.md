@PROJECT_PROMPT.md

# gitbatch — Development Guide

## Tech Stack

- **Go 1.23+**
- **CLI/Output:** [cli-kit](https://github.com/natikgadzhi/cli-kit) (cobra, table, output, progress, version)
- **Git:** shell out to `git` via `os/exec` (NOT go-git)
- **Parallelism:** `golang.org/x/sync/errgroup`

## Quality Checks

Before committing, every builder MUST verify:

```bash
go build ./...
go vet ./...
go test ./...
```

## Multi-agent Work Environment

Builds in this repo run as Workflows, Claude Code's multi-agent mode, not as agent teams. Start build sessions with `/effort ultracode` (or `claude --effort ultracode`) so each phase can run as a workflow without asking first. Without ultracode, propose the phase's workflow with a rough agent count and wait for the user to write `ultracode` or "use a workflow".

### How It Works

1. The user describes the project in `PROJECT_PROMPT.md` and `README.md`
2. The lead (you, the main session) reads them, agrees on a plan with the user, and writes it as phased task files in `tasks/backlog/`
3. Each phase runs as one workflow: builder agents implement the phase's tasks in parallel, each in its own worktree, and open a pull request; reviewer agents review each PR as soon as it opens; builders fix what reviewers flag
4. Between phases the lead stays in the loop: reads the workflow's result, merges approved PRs, keeps `main` up to date, and checks in with the user before starting the next phase

### Lead Behavior

The user will first discuss the high level plan with you. Once the user confirms we're good to start work:

1. **Read** `PROJECT_PROMPT.md` and other top-level docs to understand the project
2. **Plan** — Break the project into phases and tasks:
   - **Phase 0: Bootstrap** — Project scaffolding, directory structure, config files, dependency setup
   - **Phase 1: Core** — Core data models, types, interfaces, database schema
   - **Phase 2: Implementation** — Feature implementation (parallelize heavily here)
   - **Phase 3: Integration** — Wire components together, integration tests
   - **Phase 4: Polish** — Error handling, edge cases, documentation, final tests
3. **Write task files** in `tasks/backlog/` (see Task File System), each with:
   - Clear imperative title ("Implement user authentication endpoint")
   - Acceptance criteria, file paths, and dependencies
   - Edge cases that are covered, and edge cases that are not important
   - Tasks within one phase must be independent of each other. If task B needs task A's output, B goes in a later phase.
4. **Run the phase as a workflow** (load the `workflow-authoring` skill before writing the script):
   - `pipeline()` over the phase's task files with stages build → review → fix, so each PR goes to review as soon as it is ready. No barrier between build and review.
   - Builders run with `isolation: 'worktree'` and follow Builder Instructions below. They return the branch, PR URL, check results, and edge cases covered and not covered (use a `schema`).
   - Reviewers follow Reviewer Instructions below. Use two per PR with different lenses (correctness and tests; security and simplicity) and a higher `effort` than the builders. They post their review on the PR and return a verdict.
   - On "changes requested", a builder addresses the feedback on the same branch and the PR goes back to review. Allow at most 2 rounds, then return the PR as unresolved instead of looping.
   - Builders and reviewers inherit the session model; only set `model` if the user asks.
5. **Merge** once the workflow returns: merge approved PRs one at a time, rebasing the next branch if an earlier merge conflicts with it, and `git pull --ff` on `main` after each. Report unresolved PRs to the user.
6. **Repeat** for the next phase, and stop when all tasks are done

### Builder Instructions

Builders run inside the phase workflow, each in its own worktree. The lead passes the task file path and full task details in the prompt. Each builder MUST:

1. **Read the task file** and the existing code it touches before writing anything
2. **Branch** off the latest `main` as `task-N-description`, and move the task file from `tasks/backlog/` to `tasks/in-progress/` in the first commit
3. **Implement the task** — write code, tests, configs as needed
4. **Verify the work** — run the project's quality checks:

- `go build ./...` — builds cleanly
- `go vet ./...` — no vet errors
- `go test ./...` — all tests pass
- You are _required_ to have a strong suite of tests that follows the best practice for the project language.
- You are _required_ to run the project's linter and fix all lint errors before committing.

5. **Commit the work** with a descriptive message (include task ID):
   - Format: `[task-N] <description>`
   - One logical change per commit
6. **Move the task file** to `tasks/done/` in the PR's last commit, so merging the PR marks the task done
7. **Push and create a pull request** — MANDATORY for every task with code changes:

   ```bash
   git push -u origin task-N-description
   gh pr create --title "type(feat/doc/chore/ref) [task-N] description" --body "..."
   ```

   - Every code change goes through a PR. No direct commits to `main` are allowed.

8. **Return** the PR URL, check results, and the edge cases covered and not covered
9. **Address review feedback** on the same branch and push. Resolve all of it, unless a change would introduce a security problem the reviewer did not catch; then say so on the PR.

### Reviewer Instructions

Reviewers run inside the phase workflow with the PR number, the task file path, and their review lens in the prompt. Each reviewer MUST:

1. **Check out the PR** in its worktree with `gh pr checkout <number>`
2. **Check for conflicts** — ensure the branch rebases cleanly on latest `main`. If conflicts exist, flag them and request a rebase.
3. **Run all quality checks**:
   - `go build ./...` — builds cleanly
   - `go vet ./...` — no vet errors
   - `go test ./...` — all tests pass (including new tests for the feature)
4. **Verify the feature works** — read the task file's acceptance criteria and confirm the implementation satisfies each criterion
5. **Code quality review**:
   - Use `/simplify` Claude skill / agent
   - Read every changed file in the diff (`gh pr diff`)
   - Check for dead code, unnecessary complexity, or missing error handling
   - Verify naming conventions and code style match the rest of the codebase
   - Ensure no commented-out code was left behind
6. **Security review**:
   - Use `/security-review` Claude skill / agent
   - Ensure secrets and tokens are not logged or exposed
   - Check that user input is validated at API/CLI boundaries
   - Flag any command injection risks (e.g. unsanitized input passed to shell)
7. **Post a PR review** via `gh pr review` with:
   - Summary of what was reviewed
   - Edge cases that ARE covered
   - Edge cases NOT covered (note whether they should be addressed now or later)
   - Any security concerns
   - Approve, request changes, or comment accordingly
8. **Return** the verdict and the list of required changes

## Git Conventions

- Main checkout stays on `main` — builders and reviewers use worktrees
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
