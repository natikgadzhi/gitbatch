# Task 1: Scaffold Go project

## Objective
Set up the Go module with cli-kit, directory structure, build tooling, and a minimal working binary.

## Acceptance criteria
- `go mod init github.com/natikgadzhi/gitbatch`
- Dependencies: `github.com/natikgadzhi/cli-kit`, `golang.org/x/sync`
- Directory structure: `cmd/gitbatch/`, `internal/git/`, `internal/runner/`
- `cmd/gitbatch/main.go` with cobra root command via cli-kit
  - `--version` flag works via cli-kit/version
  - `-o/--output` flag registered via cli-kit/output
  - `-j/--jobs` flag (int, default 6)
  - `--depth` flag (int, default 3)
  - `--no-stash` flag (bool, default false)
  - Positional arg: `[directory]` (default: current working directory)
- `Makefile` with targets: `build`, `test`, `vet`
- `.goreleaser.yml` for releases
- `.github/workflows/release.yml` — single workflow: tag + goreleaser
- `README.md` with project description, install, usage
- `go build ./...` and `go vet ./...` pass
- `gitbatch --version` prints version JSON

## Notes
- Look at other cli-kit consumers (fm, gdrive-cli) for the cobra/cli-kit wiring pattern
- The root command's RunE should just print help for now — tasks 2-4 will fill in the logic
- Do NOT add bubbletea or lipgloss — this is a simple CLI, not a TUI
