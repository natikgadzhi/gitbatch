// Package runner provides parallel execution of git operations across multiple repositories.
package runner

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/natikgadzhi/gitbatch/internal/git"
	"golang.org/x/sync/errgroup"
)

// Status constants for the result of processing a repository.
const (
	StatusOK       = "OK"
	StatusUpdated  = "UPDATED"
	StatusStashed  = "STASHED"
	StatusSkipped  = "SKIPPED"
	StatusFailed   = "FAILED"
	StatusConflict = "CONFLICT"
)

// Result holds the outcome of processing a single repository.
type Result struct {
	Repo   git.Repo `json:"repo"`
	Status string   `json:"status"`
	Detail string   `json:"detail"`
	Branch string   `json:"branch"`
}

// Run processes all repos in parallel with the given concurrency limit.
// It calls onProgress (if non-nil) after each repo completes, with the count of
// completed repos. Results are returned in the same order as the input repos.
func Run(ctx context.Context, repos []git.Repo, concurrency int, noStash bool, onProgress func(done int)) []Result {
	results := make([]Result, len(repos))
	var completed atomic.Int64

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i, repo := range repos {
		i, repo := i, repo
		g.Go(func() error {
			results[i] = processRepo(ctx, repo, noStash)
			n := int(completed.Add(1))
			if onProgress != nil {
				onProgress(n)
			}
			return nil
		})
	}

	// errgroup never returns an error here since processRepo never fails the group.
	_ = g.Wait()
	return results
}

// processRepo runs the full pull workflow for a single repo.
func processRepo(_ context.Context, repo git.Repo, noStash bool) Result {
	r := Result{Repo: repo}

	// 1. Detect remote.
	remote, err := git.DetectRemote(repo.Path)
	if err != nil {
		r.Status = StatusSkipped
		r.Detail = "no remote"
		return r
	}

	// 2. Get current branch.
	branch, err := git.CurrentBranch(repo.Path)
	if err != nil {
		r.Status = StatusSkipped
		r.Detail = "detached HEAD"
		return r
	}
	r.Branch = branch

	// 3. Check dirty state.
	dirty, err := git.IsDirty(repo.Path)
	if err != nil {
		r.Status = StatusFailed
		r.Detail = fmt.Sprintf("checking dirty state: %v", err)
		return r
	}

	stashed := false
	if dirty {
		if noStash {
			r.Status = StatusSkipped
			r.Detail = "dirty worktree (--no-stash)"
			return r
		}
		if err := git.StashPush(repo.Path); err != nil {
			r.Status = StatusFailed
			r.Detail = fmt.Sprintf("stash push: %v", err)
			return r
		}
		stashed = true
	}

	// 4. Fetch.
	if err := git.Fetch(repo.Path, remote, branch); err != nil {
		// If we stashed, try to pop before returning.
		if stashed {
			_, _ = git.StashPop(repo.Path)
		}
		r.Status = StatusFailed
		r.Detail = fmt.Sprintf("fetch: %v", err)
		return r
	}

	// 5. Fast-forward merge.
	updated, err := git.MergeFF(repo.Path, remote, branch)
	if err != nil {
		if stashed {
			_, _ = git.StashPop(repo.Path)
		}
		r.Status = StatusFailed
		r.Detail = fmt.Sprintf("merge: %v", err)
		return r
	}

	// 6. Pop stash if we stashed.
	if stashed {
		conflict, err := git.StashPop(repo.Path)
		if err != nil {
			r.Status = StatusFailed
			r.Detail = fmt.Sprintf("stash pop: %v", err)
			return r
		}
		if conflict {
			r.Status = StatusConflict
			r.Detail = "stash pop conflict — stash preserved"
			return r
		}
		r.Status = StatusStashed
		r.Detail = "stashed -> pulled -> reapplied"
		return r
	}

	// 7. Determine final status.
	if updated {
		r.Status = StatusUpdated
		r.Detail = fmt.Sprintf("fast-forwarded %s/%s", remote, branch)
	} else {
		r.Status = StatusOK
		r.Detail = "already up to date"
	}
	return r
}
