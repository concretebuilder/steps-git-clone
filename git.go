package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/retry"
)

func isOriginPresent(dir, repoURL string) (bool, error) {
	absDir, err := pathutil.AbsPath(dir)
	if err != nil {
		return false, err
	}

	if file, err := os.Stat(absDir); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	} else if !file.IsDir() {
		return false, fmt.Errorf("file (%s) exists, but it's not a directory", dir)
	}

	remotes, err := runForOutput(Git.RemoteList())
	if err != nil {
		return false, err
	}

	if !strings.Contains(remotes, repoURL) {
		return false, fmt.Errorf(".git folder exists in the directory (%s), but using a different remote", dir)
	}

	return true, nil
}

func resetRepo() error {
	if err := run(Git.Reset("--hard", "HEAD")); err != nil {
		return err
	}

	if err := run(Git.Clean("-x", "-d", "f")); err != nil {
		return err
	}

	if err := run(Git.SubmoduleForeach(Git.Reset("--hard", "HEAD"))); err != nil {
		return err
	}

	return run(Git.SubmoduleForeach(Git.Clean("-x", "-d", "-f")))
}

func isPR() bool {
	return configs.PullRequestURI != "" || configs.PullRequestID != "" || configs.PullRequestMergeBranch != ""
}

func setCheckoutArg() string {
	arg := ""
	if configs.Commit != "" {
		arg = configs.Commit
	} else if configs.Tag != "" {
		arg = configs.Tag
	} else if configs.Branch != "" {
		arg = configs.Branch
	}
	return arg
}

func getDiffFile() (string, error) {
	url := fmt.Sprintf("%s/diff.txt?api_token=%s", configs.BuildURL, configs.BuildAPIToken)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	} else if resp.StatusCode != 200 {
		return "", fmt.Errorf("Can't download diff file, HTTP status code: %d", resp.StatusCode)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warnf("Failed to close response body, error: %s", err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	diffFile, err := ioutil.TempFile("", fmt.Sprintf("%s.diff", configs.PullRequestID))
	if err != nil {
		return "", err
	}

	if _, err := diffFile.Write(body); err != nil {
		return "", err
	}
	if err := diffFile.Close(); err != nil {
		return "", err
	}

	return diffFile.Name(), nil
}

func run(c *command.Model) error {
	log.Infof(c.PrintableCommandArgs())
	return c.SetStdout(os.Stdout).SetStderr(os.Stderr).Run()
}

func runForOutput(c *command.Model) (string, error) {
	return c.RunAndReturnTrimmedCombinedOutput()
}

func runWithRetry(f func() *command.Model) error {
	return retry.Times(retryCount).Wait(waitTime).Try(func(attempt uint) error {
		if attempt > 0 {
			log.Warnf("Retrying...")
		}

		err := run(f())
		if err != nil {
			log.Warnf("Attempt %d failed:", attempt+1)
			fmt.Println(err.Error())
		}

		return err
	})
}

func autoMerge(fallback bool) error {
	if configs.PullRequestMergeBranch != "" {
		if err := runWithRetry(func() *command.Model {
			return Git.Fetch("origin", configs.PullRequestMergeBranch+":"+
				strings.TrimSuffix(configs.PullRequestMergeBranch, "/merge"))
		}); err != nil {
			return fmt.Errorf("fetch Pull Request branch failed (%s), error: %v",
				configs.PullRequestMergeBranch, err)
		}

		arg := strings.TrimSuffix(configs.PullRequestMergeBranch, "/merge")
		if err := run(Git.Checkout(arg)); err != nil {
			return fmt.Errorf("checkout failed (%s), error: %v", configs.BranchDest, err)
		}
	} else if patch, err := getDiffFile(); err == nil {
		if err := run(Git.Checkout(configs.BranchDest)); err != nil {
			return fmt.Errorf("checkout failed (%s), error: %v", configs.BranchDest, err)
		}
		if err := run(Git.Apply(patch)); err != nil {
			return fmt.Errorf("can't apply patch (%s), error: %v", patch, err)
		}
	} else if fallback {
		log.Warnf("There is no Pull Request branch and can't download diff file, fallback to manual merge.")
		if err := manualMerge(false); err != nil {
			return fmt.Errorf("manual merge failed, error: %v", err)
		}
	} else {
		return fmt.Errorf("there is no Pull Request branch and can't download diff file")
	}
	return nil
}

func manualMerge(fallback bool) error {
	if err := run(Git.Checkout(configs.BranchDest)); err != nil {
		return fmt.Errorf("checkout failed (%s), error: %v", configs.BranchDest, err)
	}

	if err := run(Git.Merge(configs.Commit)); err != nil {
		if fallback {
			log.Warnf("Merge failed (%s), error: %v\nFallback to auto merge...", configs.Commit, err)
			if err := autoMerge(false); err != nil {
				return fmt.Errorf("auto merge failed, error: %v", err)
			}
		} else {
			return fmt.Errorf("merge failed (%s), error: %v", configs.Commit, err)
		}
	}

	return nil
}
