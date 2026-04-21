package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/natikgadzhi/gitbatch/internal/git"
)

// --- test helpers (same pattern as operations_test.go) ---

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "README.md"), "# test\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "initial commit")
	return dir
}

func initRepoWithRemote(t *testing.T) (string, string) {
	t.Helper()
	upstream := initRepo(t)

	bare := t.TempDir()
	os.RemoveAll(bare)
	run(t, "", "git", "clone", "--bare", upstream, bare)

	clone := t.TempDir()
	os.RemoveAll(clone)
	run(t, "", "git", "clone", bare, clone)
	run(t, clone, "git", "config", "user.email", "test@test.com")
	run(t, clone, "git", "config", "user.name", "Test")

	return clone, bare
}

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %v in %s failed: %s: %v", name, args, dir, string(out), err)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}

// pushCommitToRemote creates a second clone, commits a file, and pushes to bare.
func pushCommitToRemote(t *testing.T, bare string) {
	t.Helper()
	clone2 := t.TempDir()
	os.RemoveAll(clone2)
	run(t, "", "git", "clone", bare, clone2)
	run(t, clone2, "git", "config", "user.email", "test@test.com")
	run(t, clone2, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(clone2, "new.txt"), "new content\n")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "new commit")
	branch := run(t, clone2, "git", "symbolic-ref", "--short", "HEAD")
	run(t, clone2, "git", "push", "origin", trimSpace(branch))
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}

// --- tests ---

func TestRun_AlreadyUpToDate(t *testing.T) {
	clone, _ := initRepoWithRemote(t)

	repos := []git.Repo{
		{Path: clone, RelPath: "repo1"},
	}

	results := Run(context.Background(), repos, 2, false, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusOK {
		t.Errorf("expected status OK, got %s (%s)", results[0].Status, results[0].Detail)
	}
}

func TestRun_Updated(t *testing.T) {
	clone, bare := initRepoWithRemote(t)
	pushCommitToRemote(t, bare)

	repos := []git.Repo{
		{Path: clone, RelPath: "repo1"},
	}

	results := Run(context.Background(), repos, 2, false, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusUpdated {
		t.Errorf("expected status UPDATED, got %s (%s)", results[0].Status, results[0].Detail)
	}
}

func TestRun_SkipsNoRemote(t *testing.T) {
	repo := initRepo(t)

	repos := []git.Repo{
		{Path: repo, RelPath: "no-remote"},
	}

	results := Run(context.Background(), repos, 2, false, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusSkipped {
		t.Errorf("expected status SKIPPED, got %s (%s)", results[0].Status, results[0].Detail)
	}
}

func TestRun_NoStashSkipsDirty(t *testing.T) {
	clone, _ := initRepoWithRemote(t)
	writeFile(t, filepath.Join(clone, "dirty.txt"), "uncommitted\n")

	repos := []git.Repo{
		{Path: clone, RelPath: "dirty-repo"},
	}

	results := Run(context.Background(), repos, 2, true, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusSkipped {
		t.Errorf("expected status SKIPPED, got %s (%s)", results[0].Status, results[0].Detail)
	}
	if results[0].Detail != "dirty worktree (--no-stash)" {
		t.Errorf("expected detail about --no-stash, got %s", results[0].Detail)
	}
}

func TestRun_UntrackedOnlyDoesNotFailOnStashPop(t *testing.T) {
	clone, _ := initRepoWithRemote(t)
	// Untracked files make IsDirty report true but git stash push
	// without --include-untracked is a no-op. Previously this tripped
	// the "No stash entries found" path on the pop.
	writeFile(t, filepath.Join(clone, "leftover.txt"), "leftover\n")

	repos := []git.Repo{
		{Path: clone, RelPath: "untracked-only"},
	}

	results := Run(context.Background(), repos, 2, false, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusOK {
		t.Errorf("expected status OK, got %s (%s)", results[0].Status, results[0].Detail)
	}
}

func TestRun_MissingRemoteBranchSkipped(t *testing.T) {
	clone, _ := initRepoWithRemote(t)

	// Check out a fresh local branch whose name has no counterpart on
	// origin — the classic "feature branch merged and deleted remotely"
	// shape. Without upstream tracking configured, the old behavior
	// would fetch "origin/<localbranch>" and FAIL; new behavior is to
	// SKIP with a clear detail.
	run(t, clone, "git", "checkout", "-b", "dangling-local")

	repos := []git.Repo{
		{Path: clone, RelPath: "dangling"},
	}
	results := Run(context.Background(), repos, 2, false, nil)
	if results[0].Status != StatusSkipped {
		t.Errorf("expected SKIPPED for missing remote ref, got %s (%s)", results[0].Status, results[0].Detail)
	}
}

func TestRun_LocalBranchTracksDifferentRemoteName(t *testing.T) {
	clone, bare := initRepoWithRemote(t)

	defaultBranch, err := git.CurrentBranch(clone)
	if err != nil {
		t.Fatalf("getting branch: %v", err)
	}
	// Push a new commit to origin's default branch.
	clone2 := t.TempDir()
	os.RemoveAll(clone2)
	run(t, "", "git", "clone", bare, clone2)
	run(t, clone2, "git", "config", "user.email", "test@test.com")
	run(t, clone2, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(clone2, "new.txt"), "new content\n")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "new commit")
	run(t, clone2, "git", "push", "origin", defaultBranch)

	// Create a local branch named "feature" that tracks the default
	// branch on origin. Before the upstream-aware fix this would try to
	// fetch origin/feature and fail.
	run(t, clone, "git", "checkout", "-b", "feature", "--track", "origin/"+defaultBranch)

	repos := []git.Repo{
		{Path: clone, RelPath: "differently-named"},
	}
	results := Run(context.Background(), repos, 2, false, nil)
	if results[0].Status != StatusUpdated {
		t.Errorf("expected UPDATED, got %s (%s)", results[0].Status, results[0].Detail)
	}
	if !strings.Contains(results[0].Detail, "origin/"+defaultBranch) {
		t.Errorf("expected detail to reference origin/%s, got %q", defaultBranch, results[0].Detail)
	}
}

func TestRun_UntrackedWouldBeOverwrittenRecovers(t *testing.T) {
	clone, bare := initRepoWithRemote(t)

	branch, err := git.CurrentBranch(clone)
	if err != nil {
		t.Fatalf("getting branch: %v", err)
	}

	// Push a commit to origin that adds file "collide.txt".
	clone2 := t.TempDir()
	os.RemoveAll(clone2)
	run(t, "", "git", "clone", bare, clone2)
	run(t, clone2, "git", "config", "user.email", "test@test.com")
	run(t, clone2, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(clone2, "collide.txt"), "from remote\n")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "add collide.txt")
	run(t, clone2, "git", "push", "origin", branch)

	// In the original clone, create the same filename as an untracked file —
	// this blocks the FF merge with "untracked working tree files would be
	// overwritten". Runner should stash --include-untracked, merge, then pop.
	writeFile(t, filepath.Join(clone, "collide.txt"), "local untracked\n")

	repos := []git.Repo{
		{Path: clone, RelPath: "untracked-collide"},
	}
	results := Run(context.Background(), repos, 1, false, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Recovery stashes -u -> merges cleanly -> pop fails because the
	// stashed untracked file collides with the now-tracked path. The
	// stash is preserved and the repo is reported as CONFLICT so the
	// user can resolve manually.
	if results[0].Status != StatusConflict {
		t.Errorf("expected CONFLICT, got %s (%s)", results[0].Status, results[0].Detail)
	}
}

func TestRun_StashedAndReapplied(t *testing.T) {
	clone, bare := initRepoWithRemote(t)
	pushCommitToRemote(t, bare)

	// Make the clone dirty (different file than new.txt to avoid conflicts).
	writeFile(t, filepath.Join(clone, "local.txt"), "local changes\n")
	run(t, clone, "git", "add", "local.txt")

	repos := []git.Repo{
		{Path: clone, RelPath: "stash-repo"},
	}

	results := Run(context.Background(), repos, 2, false, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusStashed {
		t.Errorf("expected status STASHED, got %s (%s)", results[0].Status, results[0].Detail)
	}
}

func TestRun_ProgressCallback(t *testing.T) {
	clone1, _ := initRepoWithRemote(t)
	clone2, _ := initRepoWithRemote(t)

	repos := []git.Repo{
		{Path: clone1, RelPath: "repo1"},
		{Path: clone2, RelPath: "repo2"},
	}

	var maxProgress atomic.Int64
	results := Run(context.Background(), repos, 2, false, func(done int) {
		for {
			cur := maxProgress.Load()
			if int64(done) <= cur {
				break
			}
			if maxProgress.CompareAndSwap(cur, int64(done)) {
				break
			}
		}
	})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if maxProgress.Load() != 2 {
		t.Errorf("expected final progress 2, got %d", maxProgress.Load())
	}
}

func TestRun_PreservesOrder(t *testing.T) {
	clone1, _ := initRepoWithRemote(t)
	clone2, _ := initRepoWithRemote(t)
	local := initRepo(t) // no remote -> SKIPPED

	repos := []git.Repo{
		{Path: clone1, RelPath: "aaa"},
		{Path: local, RelPath: "bbb"},
		{Path: clone2, RelPath: "ccc"},
	}

	results := Run(context.Background(), repos, 1, false, nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Repo.RelPath != "aaa" {
		t.Errorf("result[0] should be aaa, got %s", results[0].Repo.RelPath)
	}
	if results[1].Repo.RelPath != "bbb" {
		t.Errorf("result[1] should be bbb, got %s", results[1].Repo.RelPath)
	}
	if results[1].Status != StatusSkipped {
		t.Errorf("result[1] should be SKIPPED, got %s", results[1].Status)
	}
	if results[2].Repo.RelPath != "ccc" {
		t.Errorf("result[2] should be ccc, got %s", results[2].Repo.RelPath)
	}
}
