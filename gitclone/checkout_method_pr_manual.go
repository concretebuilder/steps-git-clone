package gitclone

import (
	"fmt"
	"strings"

	"github.com/bitrise-io/go-utils/command/git"
	"github.com/bitrise-io/go-utils/log"
)

const forkRemoteName = "fork"

//
// PRManualMergeParams are parameters to check out a Merge Request using manual merge
type PRManualMergeParams struct {
	// Source
	HeadBranch, Commit string
	// Target
	BaseBranch string
}

//NewPRManualMergeParams validates and returns a new PRManualMergeParams
func NewPRManualMergeParams(headBranch, commit, baseBranch string) (*PRManualMergeParams, error) {
	if strings.TrimSpace(headBranch) == "" {
		return nil, NewParameterValidationError("manual PR merge checkout strategy can not be used: no head branch specified")
	}
	if strings.TrimSpace(commit) == "" {
		return nil, NewParameterValidationError("manual PR merge checkout strategy can not be used: no head branch commit hash specified")
	}
	if strings.TrimSpace(baseBranch) == "" {
		return nil, NewParameterValidationError("manual PR merge checkout strategy can not be used: no base branch specified")
	}

	return &PRManualMergeParams{
		HeadBranch: headBranch,
		Commit:     commit,
		BaseBranch: baseBranch,
	}, nil
}

// checkoutPRManualMerge
type checkoutPRManualMerge struct {
	params PRManualMergeParams
}

func (c checkoutPRManualMerge) do(gitCmd git.Git, fetchOptions fetchOptions, fallback fallbackRetry) error {
	// Fetch and checkout base (target) branch
	baseBranchRef := branchRefPrefix + c.params.BaseBranch
	if err := fetchInitialBranch(gitCmd, defaultRemoteName, baseBranchRef, fetchOptions); err != nil {
		return err
	}

	if err := manualMerge(gitCmd, fetchOptions, fallback, manualMergeParams{
		HeadBranch:  c.params.HeadBranch,
		HeadRepoURL: "",
		BaseBranch:  c.params.BaseBranch,
		MergeArg:    c.params.Commit,
	}); err != nil {
		return err
	}

	return detachHead(gitCmd)
}

//
// ForkPRManualMergeParams are parameters to check out a Pull Request using manual merge
type ForkPRManualMergeParams struct {
	// Source
	HeadBranch, HeadRepoURL string
	// Target
	BaseBranch string
}

// NewForkPRManualMergeParams validates and returns a new ForkPRManualMergeParams
func NewForkPRManualMergeParams(headBranch, forkRepoURL, baseBranch string) (*ForkPRManualMergeParams, error) {
	if strings.TrimSpace(headBranch) == "" {
		return nil, NewParameterValidationError("manual PR (fork) merge checkout strategy can not be used: no head branch specified")
	}
	if strings.TrimSpace(forkRepoURL) == "" {
		return nil, NewParameterValidationError("manual PR (fork) merge chekout strategy can not be used: no base repository URL specified")
	}
	if strings.TrimSpace(baseBranch) == "" {
		return nil, NewParameterValidationError("manual PR (fork) merge checkout strategy can not be used: no base branch specified")
	}

	return &ForkPRManualMergeParams{
		HeadBranch:  headBranch,
		HeadRepoURL: forkRepoURL,
		BaseBranch:  baseBranch,
	}, nil
}

// checkoutForkPRManualMerge
type checkoutForkPRManualMerge struct {
	params ForkPRManualMergeParams
}

func (c checkoutForkPRManualMerge) do(gitCmd git.Git, fetchOptions fetchOptions, fallback fallbackRetry) error {
	// Fetch and checkout base branch
	baseBranchRef := branchRefPrefix + c.params.BaseBranch
	if err := fetchInitialBranch(gitCmd, defaultRemoteName, baseBranchRef, fetchOptions); err != nil {
		return err
	}

	const forkRemoteName = "fork"
	remoteForkBranch := fmt.Sprintf("%s/%s", forkRemoteName, c.params.HeadBranch)
	if err := manualMerge(gitCmd, fetchOptions, fallback, manualMergeParams{
		HeadBranch:  c.params.HeadBranch,
		HeadRepoURL: c.params.HeadRepoURL,
		BaseBranch:  c.params.BaseBranch,
		MergeArg:    remoteForkBranch,
	}); err != nil {
		return err
	}

	return detachHead(gitCmd)
}

type manualMergeParams struct {
	// Source
	MergeArg    string
	HeadBranch  string
	HeadRepoURL string
	// Target
	BaseBranch string
}

func manualMerge(gitCmd git.Git, fetchOptions fetchOptions, fallback fallbackRetry, params manualMergeParams) error {
	commitHash, err := runner.RunForOutput(gitCmd.Log("%H"))
	if err != nil {
		log.Errorf("log commit hash: %v", err)
	}
	log.Printf("commit hash: %s", commitHash)

	remote := defaultRemoteName
	if params.HeadRepoURL != "" {
		remote = forkRemoteName
		// Add fork remote
		if err := runner.Run(gitCmd.RemoteAdd(forkRemoteName, params.HeadRepoURL)); err != nil {
			return fmt.Errorf("adding remote fork repository failed (%s): %v", params.HeadRepoURL, err)
		}
	}

	// Fetch + merge fork branch
	forkBranchRef := branchRefPrefix + params.HeadBranch
	if err := fetch(gitCmd, remote, forkBranchRef, fetchOptions); err != nil {
		return err
	}

	if err := mergeWithCustomRetry(gitCmd, params.MergeArg, fallback); err != nil {
		return err
	}

	return nil
}
