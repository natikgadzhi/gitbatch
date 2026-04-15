package git

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// excludedDirs contains directory names that should be skipped during discovery.
var excludedDirs = map[string]bool{
	".build":       true,
	"node_modules": true,
	".git":         true,
	"vendor":       true,
}

// Discover walks the directory tree starting from root and returns all git
// repositories found, up to maxDepth levels deep. A directory is considered a
// git repository if it contains a .git entry (directory or file).
//
// maxDepth of 0 means only the root directory itself is checked; 1 means root
// and its immediate children, and so on.
//
// The returned repos are sorted by their relative path.
func Discover(root string, maxDepth int) ([]Repo, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving root path: %w", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("accessing root directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", root)
	}

	var repos []Repo
	if err := discover(root, root, 0, maxDepth, &repos); err != nil {
		return nil, err
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].RelPath < repos[j].RelPath
	})

	return repos, nil
}

// discover recursively walks directories, collecting git repos.
func discover(root, dir string, depth, maxDepth int, repos *[]Repo) error {
	if depth > maxDepth {
		return nil
	}

	// Check if this directory is a git repo.
	gitPath := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		relPath, err := filepath.Rel(root, dir)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}
		*repos = append(*repos, Repo{
			Path:    dir,
			RelPath: relPath,
		})
		// Don't recurse into repos — they are leaf nodes.
		return nil
	}

	// Read children and recurse.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if excludedDirs[entry.Name()] {
			continue
		}
		child := filepath.Join(dir, entry.Name())
		if err := discover(root, child, depth+1, maxDepth, repos); err != nil {
			return err
		}
	}

	return nil
}
