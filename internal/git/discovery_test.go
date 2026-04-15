package git

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestTree creates a temporary directory structure for testing.
// Returns the root path and a cleanup function.
func setupTestTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create repos at various depths:
	// root/
	//   alpha/          <- repo (.git dir)
	//   beta/
	//     gamma/        <- repo (.git dir)
	//   delta/          <- repo (.git file, worktree)
	//   node_modules/
	//     hidden/       <- repo (should be skipped)
	//   .build/
	//     hidden2/      <- repo (should be skipped)
	//   vendor/
	//     hidden3/      <- repo (should be skipped)
	//   deep/
	//     level1/
	//       level2/     <- repo (.git dir)

	mkRepo := func(path string, gitFile bool) {
		t.Helper()
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		gitPath := filepath.Join(path, ".git")
		if gitFile {
			// Simulate a worktree .git file.
			if err := os.WriteFile(gitPath, []byte("gitdir: /some/other/path\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.Mkdir(gitPath, 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}

	mkRepo(filepath.Join(root, "alpha"), false)
	mkRepo(filepath.Join(root, "beta", "gamma"), false)
	mkRepo(filepath.Join(root, "delta"), true) // .git file (worktree)
	mkRepo(filepath.Join(root, "node_modules", "hidden"), false)
	mkRepo(filepath.Join(root, ".build", "hidden2"), false)
	mkRepo(filepath.Join(root, "vendor", "hidden3"), false)
	mkRepo(filepath.Join(root, "deep", "level1", "level2"), false)

	return root
}

func TestDiscoverNestedRepos(t *testing.T) {
	root := setupTestTree(t)

	repos, err := Discover(root, 10)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	want := []string{"alpha", "beta/gamma", "deep/level1/level2", "delta"}
	if len(repos) != len(want) {
		t.Fatalf("got %d repos, want %d: %v", len(repos), len(want), relPaths(repos))
	}

	for i, w := range want {
		if repos[i].RelPath != w {
			t.Errorf("repos[%d].RelPath = %q, want %q", i, repos[i].RelPath, w)
		}
	}
}

func TestDiscoverDepthLimit(t *testing.T) {
	root := setupTestTree(t)

	// depth=1: root + one level down — should find alpha, delta (depth 1),
	// but not beta/gamma (depth 2) or deep/level1/level2 (depth 3).
	repos, err := Discover(root, 1)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	want := []string{"alpha", "delta"}
	if len(repos) != len(want) {
		t.Fatalf("got %d repos, want %d: %v", len(repos), len(want), relPaths(repos))
	}

	for i, w := range want {
		if repos[i].RelPath != w {
			t.Errorf("repos[%d].RelPath = %q, want %q", i, repos[i].RelPath, w)
		}
	}
}

func TestDiscoverExcludedDirs(t *testing.T) {
	root := setupTestTree(t)

	repos, err := Discover(root, 10)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	excluded := map[string]bool{
		"node_modules/hidden": true,
		".build/hidden2":      true,
		"vendor/hidden3":      true,
	}

	for _, r := range repos {
		if excluded[r.RelPath] {
			t.Errorf("excluded repo was discovered: %s", r.RelPath)
		}
	}
}

func TestDiscoverGitFile(t *testing.T) {
	root := setupTestTree(t)

	repos, err := Discover(root, 10)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	found := false
	for _, r := range repos {
		if r.RelPath == "delta" {
			found = true
			break
		}
	}
	if !found {
		t.Error("repo with .git file (worktree) was not discovered")
	}
}

func TestDiscoverEmptyDir(t *testing.T) {
	root := t.TempDir()

	repos, err := Discover(root, 10)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if len(repos) != 0 {
		t.Errorf("got %d repos, want 0", len(repos))
	}
}

func TestDiscoverNonExistentRoot(t *testing.T) {
	_, err := Discover("/tmp/definitely-does-not-exist-gitbatch-test", 10)
	if err == nil {
		t.Error("expected error for non-existent root, got nil")
	}
}

func TestDiscoverDepthZero(t *testing.T) {
	// depth=0 means only check root itself.
	root := setupTestTree(t)

	repos, err := Discover(root, 0)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	// Root is not a repo, and depth=0 means we don't descend.
	if len(repos) != 0 {
		t.Errorf("got %d repos at depth 0, want 0: %v", len(repos), relPaths(repos))
	}
}

func TestDiscoverAbsolutePath(t *testing.T) {
	root := setupTestTree(t)

	repos, err := Discover(root, 10)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	for _, r := range repos {
		if !filepath.IsAbs(r.Path) {
			t.Errorf("repo path is not absolute: %s", r.Path)
		}
	}
}

// relPaths extracts RelPath from a slice of Repo for easy printing.
func relPaths(repos []Repo) []string {
	out := make([]string, len(repos))
	for i, r := range repos {
		out[i] = r.RelPath
	}
	return out
}
