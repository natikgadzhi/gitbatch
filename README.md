# gitbatch

A simple CLI tool that batch-updates multiple git repositories in parallel.

## Install

### From source

```bash
go install github.com/natikgadzhi/gitbatch/cmd/gitbatch@latest
```

### From releases

Download a binary from the [releases page](https://github.com/natikgadzhi/gitbatch/releases).

## Usage

```
gitbatch [flags] [directory]
```

By default, gitbatch discovers git repositories under the current directory (up to 3 levels deep), fetches and fast-forward merges them all in parallel.

### Flags

```
  -j, --jobs <n>       Max parallel operations (default: 6)
      --depth <n>      Max directory depth for discovery (default: 3)
      --no-stash       Skip repos with dirty worktrees instead of stashing
  -o, --output         Output format: table, json (default: auto via TTY)
      --version        Print version
  -h, --help           Show help
```

### Examples

```bash
# Update all repos under ~/src
gitbatch ~/src

# Use 12 parallel workers, search 5 levels deep
gitbatch -j 12 --depth 5 ~/projects

# Output JSON (useful for cron / piping)
gitbatch -o json ~/src
```

## License

MIT
