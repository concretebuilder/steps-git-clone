package gitclone

import (
	"errors"
	"strings"

	"github.com/bitrise-io/go-utils/command/git"
)

//
// checkoutNone
type checkoutNone struct{}

func (c checkoutNone) do(gitCmd git.Git, fetchOptions fetchOptions) error {
	return nil
}

//
// checkoutCommit
type checkoutCommit struct {
	params CommitParams
}

func (c checkoutCommit) Validate() error {
	if strings.TrimSpace(c.params.Commit) == "" {
		return errors.New("no commit hash specified")
	}

	return nil
}

func (c checkoutCommit) do(gitCmd git.Git, fetchOptions fetchOptions) error {
	// Fetch then checkout
	// No branch specified for fetch
	if err := fetch(gitCmd, fetchOptions, nil); err != nil {
		return err
	}

	if err := checkoutWithCustomRetry(gitCmd, checkoutArg{arg: c.params.Commit}, simpleUnshallowFunc); err != nil {
		return err
	}

	return nil
}

//
// checkoutBranch
type checkoutBranch struct {
	params BranchParams
}

func (c checkoutBranch) do(gitCmd git.Git, fetchOptions fetchOptions) error {
	branchRef := *newOriginFetchRef(branchRefPrefix + c.params.Branch)
	if err := fetchInitialBranch(gitCmd, branchRef, fetchOptions); err != nil {
		return err
	}

	return nil
}

//
// checkoutTag
type checkoutTag struct {
	params TagParams
}

func (c checkoutTag) do(gitCmd git.Git, fetchOptions fetchOptions) error {
	var branchRef *fetchRef
	if c.params.Branch != nil {
		branchRef = newOriginFetchRef(branchRefPrefix + *c.params.Branch)
	}

	if err := fetch(gitCmd, fetchOptions, branchRef); err != nil {
		return err
	}

	if err := checkoutWithCustomRetry(gitCmd, checkoutArg{arg: c.params.Tag}, simpleUnshallowFunc); err != nil {
		return err
	}

	return nil
}
