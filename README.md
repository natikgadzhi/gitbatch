# gitbatch

A simple CLI tool that batch-updates multiple git repositories in parallel.

## Install

### Homebrew

```bash
brew install natikgadzhi/taps/gitbatch
```

### From source

```bash
go install github.com/natikgadzhi/gitbatch/cmd/gitbatch@latest
```

### From releases

Download a binary from the [releases page](https://github.com/natikgadzhi/gitbatch/releases).

## Usage

```
gitbatch <command> [flags]
```

### `gitbatch sync`

Discovers git repositories under a directory, fetches and fast-forward merges them all in parallel. For dirty worktrees, stashes changes, pulls, and reapplies.

```
gitbatch sync [flags] [directory]
```

```
  -j, --jobs <n>       Max parallel operations (default: 6)
      --depth <n>      Max directory depth for discovery (default: 3)
      --no-stash       Skip repos with dirty worktrees instead of stashing
```

### Global flags

```
  -o, --output         Output format: table, json (default: auto via TTY)
      --version        Print version
  -h, --help           Show help
```

### `gitbatch schedule`

Set up a macOS LaunchAgent to run sync automatically.

```
gitbatch schedule set [flags] [directory]
gitbatch schedule show
gitbatch schedule remove
gitbatch schedule run
gitbatch schedule logs
```

```
      --time <HH:MM>   Run daily at a specific time (default: 08:00)
      --every <dur>    Run on an interval (e.g. 4h, 30m, 1h30m)
```

`--time` and `--every` are mutually exclusive.

### Examples

```bash
# Sync all repos under ~/src
gitbatch sync ~/src

# Use 12 parallel workers, search 5 levels deep
gitbatch sync -j 12 --depth 5 ~/projects

# Output JSON (useful for cron / piping)
gitbatch sync -o json ~/src

# Sync ~/src every morning at 8am
gitbatch schedule set --time 08:00 ~/src

# Sync every 4 hours
gitbatch schedule set --every 4h ~/src

# Check schedule status
gitbatch schedule show

# View recent logs
gitbatch schedule logs
```

## License

MIT
