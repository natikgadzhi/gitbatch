# Task 2: Repository discovery

## Objective
Implement repo discovery: walk a directory tree, find git repositories, return them as a sorted list.

## Depends on
- Task 1 (scaffold — need the module and directory structure)

## Acceptance criteria
- `internal/git/discovery.go`:
  - `Discover(root string, maxDepth int) ([]Repo, error)`
  - Walks directory tree starting from `root`
  - A directory is a repo if it contains `.git` (directory or file — worktrees use a file)
  - Skips directories named: `.build`, `node_modules`, `.git`, `vendor`
  - Respects `maxDepth` (0 = root only, 1 = one level down, etc.)
  - Returns repos sorted by relative path
- `internal/git/repo.go`:
  - `Repo` struct with: `Path string` (absolute), `RelPath string` (relative to root)
  - `Status` and `Detail` fields for later use by runner
- `internal/git/discovery_test.go`:
  - Test: discovers repos in nested directories
  - Test: respects depth limit
  - Test: skips excluded directories (.build, node_modules, vendor)
  - Test: handles `.git` file (worktree) same as `.git` directory
  - Test: returns empty slice for directory with no repos
  - Test: returns error for non-existent root directory
- `go test ./internal/git/...` passes

## Notes
- Use `os.ReadDir` for walking — no need for `filepath.Walk` since we need depth control
- Keep it simple: recursive function with depth counter
