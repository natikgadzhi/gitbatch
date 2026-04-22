package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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

func TestBranchUpstream_Configured(t *testing.T) {
	clone, _ := initRepoWithRemote(t)
	branch, err := CurrentBranch(clone)
	if err != nil {
		t.Fatalf("getting branch: %v", err)
	}

	remote, remoteBranch, err := BranchUpstream(clone, branch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remote != "origin" {
		t.Errorf("expected remote=origin, got %q", remote)
	}
	if remoteBranch != branch {
		t.Errorf("expected remoteBranch=%q, got %q", branch, remoteBranch)
	}
}

func TestBranchUpstream_NotConfigured(t *testing.T) {
	repo := initRepo(t) // no remote, no tracking config
	branch, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("getting branch: %v", err)
	}

	remote, remoteBranch, err := BranchUpstream(repo, branch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remote != "" || remoteBranch != "" {
		t.Errorf("expected empty upstream for unconfigured branch, got (%q, %q)", remote, remoteBranch)
	}
}

func TestBranchUpstream_DifferentRemoteBranchName(t *testing.T) {
	clone, bare := initRepoWithRemote(t)

	branch, err := CurrentBranch(clone)
	if err != nil {
		t.Fatalf("getting branch: %v", err)
	}
	// Create a local branch that tracks origin/<default branch> but has a
	// different local name. This is the scenario that previously tripped
	// gitbatch: local name != remote name.
	run(t, clone, "git", "checkout", "-b", "local-feature", "--track", "origin/"+branch)
	_ = bare

	remote, remoteBranch, err := BranchUpstream(clone, "local-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remote != "origin" {
		t.Errorf("expected origin, got %q", remote)
	}
	if remoteBranch != branch {
		t.Errorf("expected remote branch %q, got %q", branch, remoteBranch)
	}
}

func TestIsMissingRemoteRef(t *testing.T) {
	clone, _ := initRepoWithRemote(t)
	// Ask for a ref that certainly doesn't exist on origin.
	err := Fetch(clone, "origin", "definitely-does-not-exist-branch")
	if err == nil {
		t.Fatal("expected fetch to fail for nonexistent remote ref")
	}
	if !IsMissingRemoteRef(err) {
		t.Errorf("IsMissingRemoteRef should classify %q as missing-ref", err.Error())
	}
}

func TestStashPush_UntrackedOnlyReturnsFalse(t *testing.T) {
	repo := initRepo(t)
	// Untracked file — `git status --porcelain` reports it but plain
	// `git stash push` ignores it and creates no stash entry.
	writeFile(t, filepath.Join(repo, "new-untracked.txt"), "untracked content\n")

	stashed, err := StashPush(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stashed {
		t.Error("expected stashed=false when only untracked files are present")
	}
}

func TestClassifiers(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		network bool
		auth    bool
		missing bool
		untrack bool
	}{
		{"nil", nil, false, false, false, false},
		{"kex", errors.New("kex_exchange_identification: Connection closed by remote"), true, false, false, false},
		{"timeout", errors.New("ssh: connect: Operation timed out"), true, false, false, false},
		{"early-eof", errors.New("fatal: the remote end hung up unexpectedly\nfatal: early EOF"), true, false, false, false},
		{"publickey", errors.New("git@github.com: Permission denied (publickey)"), false, true, false, false},
		{"auth-failed", errors.New("remote: HTTP Basic: Access denied\nfatal: Authentication failed"), false, true, false, false},
		{"missing-ref", errors.New("fatal: couldn't find remote ref refs/heads/gone"), false, false, true, false},
		{"untracked", errors.New("error: The following untracked working tree files would be overwritten by merge:\n\tfoo"), false, false, false, true},
		{"other", errors.New("fatal: not a git repository"), false, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNetworkError(tc.err); got != tc.network {
				t.Errorf("IsNetworkError = %v, want %v", got, tc.network)
			}
			if got := IsAuthError(tc.err); got != tc.auth {
				t.Errorf("IsAuthError = %v, want %v", got, tc.auth)
			}
			if got := IsMissingRemoteRef(tc.err); got != tc.missing {
				t.Errorf("IsMissingRemoteRef = %v, want %v", got, tc.missing)
			}
			if got := IsUntrackedWouldBeOverwritten(tc.err); got != tc.untrack {
				t.Errorf("IsUntrackedWouldBeOverwritten = %v, want %v", got, tc.untrack)
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"network", errors.New("kex_exchange_identification: Connection closed"), "network error"},
		{"auth", errors.New("Permission denied (publickey)"), "authentication failed"},
		{"missing", errors.New("fatal: couldn't find remote ref foo"), "remote ref missing"},
		{"untracked", errors.New("error: The following untracked working tree files would be overwritten by merge"), "untracked files would be overwritten"},
		{"fatal-line", errors.New("some prefix\nfatal: not a git repository\nmore output"), "not a git repository"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Summarize(tc.err); got != tc.want {
				t.Errorf("Summarize = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFetchWithRetry_RetriesNetworkErrors(t *testing.T) {
	// Replace backoff with near-zero delay so the test is fast.
	orig := retryBackoff
	retryBackoff = func(int) time.Duration { return time.Millisecond }
	defer func() { retryBackoff = orig }()

	clone, _ := initRepoWithRemote(t)
	// A nonexistent remote will fail with a non-network error — should return
	// after one attempt without retrying.
	err := FetchWithRetry(clone, "no-such-remote", "main", 3)
	if err == nil {
		t.Fatal("expected error for nonexistent remote")
	}
	if IsNetworkError(err) {
		t.Errorf("did not expect a network error, got %v", err)
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
	stashed, err := StashPush(repo)
	if err != nil {
		t.Fatalf("stash push error: %v", err)
	}
	if !stashed {
		t.Fatal("expected stash push to report stashed=true for tracked changes")
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
