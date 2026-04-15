package git

import (
	"fmt"
	"os/exec"
	"strings"
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
func StashPush(repoPath string) error {
	_, err := gitCmd(repoPath, "stash", "push", "-m", "gitbatch auto-stash")
	if err != nil {
		return fmt.Errorf("stashing changes in %s: %w", repoPath, err)
	}
	return nil
}

// StashPop pops the most recent stash entry. If the pop results in merge
// conflicts, conflict is true and err is nil (the stash is left applied with
// conflicts). A hard failure returns a non-nil error.
func StashPop(repoPath string) (conflict bool, err error) {
	output, cmdErr := gitCmd(repoPath, "stash", "pop")
	if cmdErr != nil {
		// git stash pop exits non-zero on conflicts but still applies the changes.
		// Detect conflicts by looking for the telltale output.
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "conflict") {
			return true, nil
		}
		return false, fmt.Errorf("popping stash in %s: %w", repoPath, cmdErr)
	}
	return false, nil
}
