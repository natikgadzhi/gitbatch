package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo creates a new git repo in a temp directory with one commit.
// Returns the repo path.
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

// initRepoWithRemote creates a "remote" bare repo and a clone that has it set
// as origin. Returns (clonePath, barePath).
func initRepoWithRemote(t *testing.T) (string, string) {
	t.Helper()
	// Create a repo to serve as the remote.
	upstream := initRepo(t)

	// Create a bare clone to act as the remote.
	bare := t.TempDir()
	// Remove the temp dir so git clone can create it.
	os.RemoveAll(bare)
	run(t, "", "git", "clone", "--bare", upstream, bare)

	// Clone from bare into a working copy.
	clone := t.TempDir()
	os.RemoveAll(clone)
	run(t, "", "git", "clone", bare, clone)
	run(t, clone, "git", "config", "user.email", "test@test.com")
	run(t, clone, "git", "config", "user.name", "Test")

	return clone, bare
}

// run executes a command in the given directory and fails the test on error.
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

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}

func TestDetectRemote_Origin(t *testing.T) {
	clone, _ := initRepoWithRemote(t)
	remote, err := DetectRemote(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remote != "origin" {
		t.Errorf("expected origin, got %s", remote)
	}
}

func TestDetectRemote_Upstream(t *testing.T) {
	clone, bare := initRepoWithRemote(t)
	// Rename origin to upstream.
	run(t, clone, "git", "remote", "rename", "origin", "upstream")
	_ = bare
	remote, err := DetectRemote(clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remote != "upstream" {
		t.Errorf("expected upstream, got %s", remote)
	}
}

func TestDetectRemote_NoRemote(t *testing.T) {
	repo := initRepo(t)
	_, err := DetectRemote(repo)
	if err == nil {
		t.Fatal("expected error for repo with no remotes")
	}
}

func TestCurrentBranch(t *testing.T) {
	repo := initRepo(t)
	branch, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default branch could be main or master depending on git config.
	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

func TestIsDirty_Clean(t *testing.T) {
	repo := initRepo(t)
	dirty, err := IsDirty(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dirty {
		t.Error("expected clean repo to not be dirty")
	}
}

func TestIsDirty_Dirty(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "dirty.txt"), "uncommitted\n")
	dirty, err := IsDirty(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dirty {
		t.Error("expected dirty repo to be dirty")
	}
}

func TestFetch(t *testing.T) {
	clone, bare := initRepoWithRemote(t)
	_ = bare

	branch, err := CurrentBranch(clone)
	if err != nil {
		t.Fatalf("getting branch: %v", err)
	}

	err = Fetch(clone, "origin", branch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeFF_Updated(t *testing.T) {
	clone, bare := initRepoWithRemote(t)

	branch, err := CurrentBranch(clone)
	if err != nil {
		t.Fatalf("getting branch: %v", err)
	}

	// Push a new commit to the bare remote from a second clone.
	clone2 := t.TempDir()
	os.RemoveAll(clone2)
	run(t, "", "git", "clone", bare, clone2)
	run(t, clone2, "git", "config", "user.email", "test@test.com")
	run(t, clone2, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(clone2, "new.txt"), "new content\n")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "new commit")
	run(t, clone2, "git", "push", "origin", branch)

	// Fetch and merge in the original clone.
	err = Fetch(clone, "origin", branch)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	updated, err := MergeFF(clone, "origin", branch)
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}
	if !updated {
		t.Error("expected merge to report updated=true")
	}
}

func TestMergeFF_AlreadyUpToDate(t *testing.T) {
	clone, _ := initRepoWithRemote(t)

	branch, err := CurrentBranch(clone)
	if err != nil {
		t.Fatalf("getting branch: %v", err)
	}

	err = Fetch(clone, "origin", branch)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	updated, err := MergeFF(clone, "origin", branch)
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}
	if updated {
		t.Error("expected merge to report updated=false when already up to date")
	}
}

func TestStashPushPop_RoundTrip(t *testing.T) {
	repo := initRepo(t)

	// Create dirty state.
	writeFile(t, filepath.Join(repo, "dirty.txt"), "dirty content\n")
	run(t, repo, "git", "add", "dirty.txt")

	dirty, err := IsDirty(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dirty {
		t.Fatal("expected repo to be dirty before stash")
	}

	// Stash.
	err = StashPush(repo)
	if err != nil {
		t.Fatalf("stash push error: %v", err)
	}

	dirty, err = IsDirty(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dirty {
		t.Error("expected repo to be clean after stash push")
	}

	// Pop.
	conflict, err := StashPop(repo)
	if err != nil {
		t.Fatalf("stash pop error: %v", err)
	}
	if conflict {
		t.Error("expected no conflict on stash pop")
	}

	dirty, err = IsDirty(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dirty {
		t.Error("expected repo to be dirty again after stash pop")
	}
}
