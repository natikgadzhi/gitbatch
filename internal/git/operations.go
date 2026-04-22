package git

import (
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
	"time"
)

// gitCmd runs git -C <repoPath> with the given arguments, captures combined
// stdout+stderr, and returns the trimmed output. On non-zero exit, the error
// includes the repo path and full output for debugging.
func gitCmd(repoPath string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("git %s in %s: %s: %w", strings.Join(args, " "), repoPath, output, err)
	}
	return output, nil
}

// DetectRemote returns the preferred remote for the repository. It prefers
// "origin", falls back to "upstream", and returns an error if neither exists.
func DetectRemote(repoPath string) (string, error) {
	output, err := gitCmd(repoPath, "remote")
	if err != nil {
		return "", fmt.Errorf("listing remotes for %s: %w", repoPath, err)
	}

	remotes := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			remotes[name] = true
		}
	}

	if remotes["origin"] {
		return "origin", nil
	}
	if remotes["upstream"] {
		return "upstream", nil
	}
	return "", fmt.Errorf("no origin or upstream remote in %s", repoPath)
}

// BranchUpstream returns the configured upstream for the given local
// branch as (remote, remoteBranch). Returns ("", "", nil) with no error
// when the branch has no upstream configured — callers are expected to
// fall back to a default resolution. Returns a non-nil error only on
// unexpected git failures.
//
// Respecting the branch's tracking configuration is the right default
// for sync: a local feature branch may track origin/main, a fork's
// branch of a different name, or a remote named "myfork" instead of
// "origin". Assuming <remote>/<localbranch> misses all of those.
func BranchUpstream(repoPath, branch string) (string, string, error) {
	remote, err := gitCmd(repoPath, "config", "--get", "branch."+branch+".remote")
	if err != nil {
		// `git config --get` exits 1 when the key is unset. That's not a
		// real error — it just means no upstream is configured.
		if isConfigUnset(err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("reading branch.%s.remote in %s: %w", branch, repoPath, err)
	}
	mergeRef, err := gitCmd(repoPath, "config", "--get", "branch."+branch+".merge")
	if err != nil {
		if isConfigUnset(err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("reading branch.%s.merge in %s: %w", branch, repoPath, err)
	}
	remote = strings.TrimSpace(remote)
	mergeRef = strings.TrimSpace(mergeRef)
	if remote == "" || mergeRef == "" {
		return "", "", nil
	}
	remoteBranch := strings.TrimPrefix(mergeRef, "refs/heads/")
	return remote, remoteBranch, nil
}

// isConfigUnset reports whether err came from `git config --get` exiting
// with status 1 (the documented code for "key not set"). Anything else
// is a genuine error.
func isConfigUnset(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == 1
	}
	// gitCmd wraps ExitError via fmt.Errorf; errors.As unwraps.
	return false
}

// IsMissingRemoteRef reports whether err looks like a fetch against a
// remote branch that no longer exists (e.g. deleted after a PR merge).
// These are expected in day-to-day repos and should not count as
// failures — the local branch is simply stale.
func IsMissingRemoteRef(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "couldn't find remote ref") ||
		strings.Contains(msg, "Couldn't find remote ref")
}

// IsNetworkError reports whether err looks like a transient network
// problem (SSH handshake failures, broken pipes, timeouts) as opposed
// to a permanent condition. Callers use this to decide whether a retry
// is worth attempting.
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// "Could not read from remote repository" shows up for both network
	// blips and permanent config issues (bad remote name, no access).
	// Skip the generic-sounding cases where git tells us the remote
	// itself is bogus — those won't heal on retry.
	if strings.Contains(msg, "does not appear to be a git repository") {
		return false
	}
	markers := []string{
		"Connection closed",
		"Connection timed out",
		"Operation timed out",
		"Broken pipe",
		"banner exchange",
		"kex_exchange_identification",
		"expected flush after ref listing",
		"Could not read from remote repository",
		"early EOF",
		"RPC failed",
		"remote end hung up",
	}
	for _, m := range markers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

// IsAuthError reports whether err looks like an authentication failure.
// Retries won't help here; the user needs to fix their credentials.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Permission denied (publickey)") ||
		strings.Contains(msg, "Authentication failed") ||
		strings.Contains(msg, "authentication failed")
}

// IsUntrackedWouldBeOverwritten reports whether err is the specific
// merge-abort case where an incoming commit touches an untracked path.
// Runner recovers by stashing with --include-untracked and retrying.
func IsUntrackedWouldBeOverwritten(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "untracked working tree files would be overwritten")
}

// Summarize returns a short, human-readable description of err suitable
// for the Detail column of a results table. Full error output (with
// paths, nested command invocations, and exit codes) is useful in logs
// but noisy in a summary — `Summarize` extracts the essential reason.
func Summarize(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case IsNetworkError(err):
		return "network error"
	case IsAuthError(err):
		return "authentication failed"
	case IsMissingRemoteRef(err):
		return "remote ref missing"
	case IsUntrackedWouldBeOverwritten(err):
		return "untracked files would be overwritten"
	}
	return firstInterestingLine(err.Error())
}

// firstInterestingLine returns the most useful short line from a
// multi-line git error — typically the first "fatal: ..." or
// "error: ..." message. Falls back to a truncated first line.
func firstInterestingLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "fatal: ") {
			return strings.TrimPrefix(line, "fatal: ")
		}
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "error: ") {
			return strings.TrimPrefix(line, "error: ")
		}
	}
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	const maxLen = 80
	if len(s) > maxLen {
		s = s[:maxLen-1] + "…"
	}
	return s
}

// CurrentBranch returns the current branch name via git symbolic-ref.
// Returns an error if HEAD is detached.
func CurrentBranch(repoPath string) (string, error) {
	output, err := gitCmd(repoPath, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting current branch for %s (detached HEAD?): %w", repoPath, err)
	}
	return output, nil
}

// IsDirty returns true if the working tree has uncommitted changes.
func IsDirty(repoPath string) (bool, error) {
	output, err := gitCmd(repoPath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("checking dirty state for %s: %w", repoPath, err)
	}
	return output != "", nil
}

// Fetch runs git fetch for the given remote and branch.
func Fetch(repoPath, remote, branch string) error {
	_, err := gitCmd(repoPath, "fetch", remote, branch)
	if err != nil {
		return fmt.Errorf("fetching %s/%s in %s: %w", remote, branch, repoPath, err)
	}
	return nil
}

// retryBackoff returns the delay before attempt N (0-indexed) for the
// fetch retry loop. Exponential base with a ±25% jitter keeps many
// parallel repos from synchronously hammering the remote on the same
// schedule after a shared outage. Overridable for tests.
var retryBackoff = func(attempt int) time.Duration {
	base := time.Duration(2<<uint(attempt)) * time.Second // 2s, 4s, 8s, ...
	jitter := time.Duration(rand.Int63n(int64(base / 4)))
	return base + jitter
}

// FetchWithRetry is like Fetch but retries transient network failures
// with exponential backoff. It stops immediately on non-network errors
// (e.g. a missing remote ref, which won't heal by waiting).
func FetchWithRetry(repoPath, remote, branch string, attempts int) error {
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for i := 0; i < attempts; i++ {
		err = Fetch(repoPath, remote, branch)
		if err == nil {
			return nil
		}
		if !IsNetworkError(err) {
			return err
		}
		if i == attempts-1 {
			break
		}
		time.Sleep(retryBackoff(i))
	}
	return err
}

// MergeFF attempts a fast-forward merge of remote/branch into the current
// branch. It returns true if the merge updated HEAD, false if already up to
// date. Returns an error if the merge cannot be fast-forwarded.
func MergeFF(repoPath, remote, branch string) (bool, error) {
	ref := remote + "/" + branch
	output, err := gitCmd(repoPath, "merge", "--ff-only", ref)
	if err != nil {
		return false, fmt.Errorf("fast-forward merge of %s in %s: %w", ref, repoPath, err)
	}
	// git merge prints "Already up to date." when there is nothing to do.
	if strings.Contains(output, "Already up to date") {
		return false, nil
	}
	return true, nil
}

// StashPush stashes uncommitted changes with a recognizable message.
// Returns (true, nil) when a stash entry was actually created, or
// (false, nil) when git reports "No local changes to save" — which
// happens when the worktree is only dirty with untracked files (these
// are considered dirty by `git status --porcelain` but are ignored by
// `git stash push` without --include-untracked). Treating the no-op
// case as stashed=false prevents a bogus `git stash pop` from running
// against a stash that was never created.
func StashPush(repoPath string) (bool, error) {
	return stashPush(repoPath, false)
}

// StashPushUntracked stashes tracked changes plus untracked files. Used
// as a second-chance recovery when a fast-forward merge aborts because
// an untracked path would be overwritten — the untracked file gets
// tucked away, the merge proceeds, and the stash is popped afterward.
func StashPushUntracked(repoPath string) (bool, error) {
	return stashPush(repoPath, true)
}

func stashPush(repoPath string, includeUntracked bool) (bool, error) {
	args := []string{"stash", "push", "-m", "gitbatch auto-stash"}
	if includeUntracked {
		args = append(args, "--include-untracked")
	}
	output, err := gitCmd(repoPath, args...)
	if err != nil {
		return false, fmt.Errorf("stashing changes in %s: %w", repoPath, err)
	}
	if strings.Contains(output, "No local changes to save") {
		return false, nil
	}
	return true, nil
}

// StashPop pops the most recent stash entry. If the pop results in merge
// conflicts — or refuses because an untracked file in the stash would
// overwrite a now-tracked path — conflict is true and err is nil; the
// stash is preserved either way so the user can recover manually. A
// hard failure returns a non-nil error.
func StashPop(repoPath string) (conflict bool, err error) {
	output, cmdErr := gitCmd(repoPath, "stash", "pop")
	if cmdErr != nil {
		// git stash pop exits non-zero on conflicts but still applies the changes.
		// Detect conflicts by looking for the telltale output.
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "conflict") {
			return true, nil
		}
		// Stashes created with --include-untracked fail the pop when the
		// post-merge worktree already contains a file of the same name.
		// Git leaves the stash intact; treat this as a conflict so the
		// caller preserves the stash and reports it clearly.
		if strings.Contains(output, "could not restore untracked files from stash") {
			return true, nil
		}
		return false, fmt.Errorf("popping stash in %s: %w", repoPath, cmdErr)
	}
	return false, nil
}
