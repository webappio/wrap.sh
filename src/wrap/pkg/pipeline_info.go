package wrap

import (
	"fmt"
	"github.com/layer-devops/wrap.sh/src/protocol"
	"github.com/pkg/errors"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

/* returns the first non-empty argument */
func coalesce(args ...string) string {
	for _, arg := range args {
		if arg != "" {
			return arg
		}
	}
	return ""
}

/* returns the first non-empty value
between the ENV variables with the given names */
func coalesceEnv(varNames ...string) string {
	for _, name := range varNames {
		if os.Getenv(name) != "" {
			return os.Getenv(name)
		}
	}
	return ""
}

type providerHandler struct {
	Name       string
	IsDetected func() bool
	FindInfo   func(*protocol.Hello) error
}

/* returns a function which checks whether the given
env variables have non-empty values*/
func checkForEnvVars(varNames ...string) func() bool {
	return func() bool {
		for _, varName := range varNames {
			if os.Getenv(varName) == "" {
				return false
			}
		}
		return true
	}
}

/* returns a function which checks whether the given
env variables have values matching the provided ones.

Uses "*" as a wildcard, matching any non-empty value for
an environment variable.*/
func checkForEnvVarsMap(vars map[string]string) func() bool {
	return func() bool {
		for key, value := range vars {
			//TODO: hack
			if value == "*" && os.Getenv(key) != "" {
				continue
			}
			if os.Getenv(key) != value {
				return false
			}
		}
		return true
	}
}

// Many of the below methods are based on the excellent codecov bash uploader
// You can find its' source at the following git repository:
// https://github.com/codecov/codecov-bash/blob/master/codecov

// If your provider is not supported, or if certain useful information is missing,
// you may open a PR with the fixes or an issue outlining what should be changed.

var providers = []*providerHandler{
	{
		Name: "LayerCI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":      "true",
			"LAYERCI": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("LAYERCI_BRANCH")
			h.CommitHash = os.Getenv("GIT_COMMIT")
			h.JobId = os.Getenv("LAYERCI_JOB_ID")
			//TODO: check
			h.BuildId = os.Getenv("LAYERCI_RUNNER_ID")
			h.BuildUrl = fmt.Sprintf(
				"https://layerci.com/%v/%v/%v/%v",
				os.Getenv("LAYERCI_ORG_NAME"),
				os.Getenv("LAYERCI_REPO_NAME"),
				os.Getenv("LAYERCI_JOB_ID"),
				os.Getenv("LAYERCI_RUNNER_ID"),
			)
			h.Slug = os.Getenv("LAYERCI_REPO_OWNER") + "/" + os.Getenv("LAYERCI_REPO_NAME")
			h.PullRequest = os.Getenv("LAYERCI_PULL_REQUEST")
			//TODO: check
			h.Tag = os.Getenv("GIT_TAG")
			return nil
		},
	},
	{
		Name:       "Jenkins CI",
		IsDetected: checkForEnvVars("JENKINS_URL"),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = coalesceEnv("ghprbSourceBranch", "GIT_BRANCH", "BRANCH_NAME")
			h.CommitHash = coalesceEnv("ghprbActualCommit", "GIT_COMMIT")
			h.PullRequest = coalesceEnv("ghprbPullId", "CHANGE_ID")
			h.BuildId = os.Getenv("BUILD_NUMBER")
			h.BuildUrl = os.Getenv("BUILD_URL")
			return nil
		},
	},
	{
		Name: "Travis CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":        "true",
			"TRAVIS":    "true",
			"SHIPPABLE": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			if os.Getenv("TRAVIS_BRANCH") != os.Getenv("TRAVIS_TAG") {
				h.BranchName = coalesceEnv("TRAVIS_PULL_REQUEST_BRANCH", "TRAVIS_BRANCH")
			}
			h.CommitHash = coalesceEnv("TRAVIS_PULL_REQUEST_SHA", "TRAVIS_COMMIT")
			h.PullRequest = os.Getenv("TRAVIS_PULL_REQUEST")
			h.BuildId = os.Getenv("TRAVIS_JOB_NUMBER")
			h.JobId = os.Getenv("TRAVIS_JOB_ID")
			h.Slug = os.Getenv("TRAVIS_REPO_SLUG")
			h.Tag = os.Getenv("TRAVIS_TAG")
			// TODO: some extra stuff here
			//env="$env,TRAVIS_OS_NAME"
			//language=$(compgen -A variable | grep "^TRAVIS_.*_VERSION$" | head -1)
			//if [ "$language" != "" ];
			//then
			//env="$env,${!language}"
			//fi
			return nil
		},
	},
	{
		Name:       "AWS Codebuild",
		IsDetected: checkForEnvVarsMap(map[string]string{"CODEBUILD_CI": "true"}),
		FindInfo: func(h *protocol.Hello) error {
			h.CommitHash = os.Getenv("CODEBUILD_RESOLVED_SOURCE_VERSION")
			h.BuildId = os.Getenv("CODEBUILD_BUILD_ID")
			h.BranchName = strings.ReplaceAll(
				os.Getenv("CODEBUILD_WEBHOOK_HEAD_REF"),
				"refs/heads/",
				"",
			)
			h.JobId = os.Getenv("CODEBUILD_BUILD_ID")
			slugUrl, err := url.Parse(os.Getenv("CODEBUILD_SOURCE_REPO_URL"))
			if err == nil {
				h.Slug = strings.TrimSuffix(slugUrl.Path, ".git")
			}
			//TODO
			//if [ "${CODEBUILD_SOURCE_VERSION/pr}" = "$CODEBUILD_SOURCE_VERSION" ] ; then
			//pr="false"
			//else
			//pr="$(echo "$CODEBUILD_SOURCE_VERSION" | sed 's/^pr\///')"
			//fi
			return nil
		},
	},
	{
		Name:       "Docker",
		IsDetected: checkForEnvVars("DOCKER_REPO"),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("SOURCE_BRANCH")
			h.CommitHash = os.Getenv("SOURCE_COMMIT")
			h.Slug = os.Getenv("DOCKER_REPO")
			h.Tag = os.Getenv("CACHE_TAG")
			return nil
		},
	},
	{
		Name:       "Codefresh CI",
		IsDetected: checkForEnvVars("CF_BUILD_URL", "CF_BUILD_ID"),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("CF_BRANCH")
			h.BuildId = os.Getenv("CF_BUILD_ID")
			h.BuildUrl = os.Getenv("CF_BUILD_URL")
			h.CommitHash = os.Getenv("CF_REVISION")
			return nil
		},
	},
	{
		Name:       "TeamCity CI",
		IsDetected: checkForEnvVars("TEAMCITY_VERSION"),
		FindInfo: func(h *protocol.Hello) error {
			//TODO: teamcity does not actually expose anything automatically
			// but we can get data if they're setup for codecov
			h.BranchName = os.Getenv("TEAMCITY_BUILD_BRANCH")
			h.BuildId = os.Getenv("TEAMCITY_BUILD_ID")
			h.BuildUrl = os.Getenv("TEAMCITY_BUILD_URL")
			h.CommitHash = coalesceEnv("TEAMCITY_BUILD_COMMIT", "BUILD_VCS_NUMBER")
			// TODO: why does codecov collect this
			//remote_addr="$TEAMCITY_BUILD_REPOSITORY"
			return nil
		},
	},
	{
		Name: "Circle CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":       "true",
			"CIRCLECI": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("CIRCLE_BRANCH")
			h.CommitHash = os.Getenv("CIRCLE_SHA1")
			//pr="${CIRCLE_PULL_REQUEST##*/}"
			h.PullRequest = strings.Split(os.Getenv("CIRCLE_PULL_REQUEST"), "/")[1]
			h.BuildId = os.Getenv("CIRCLE_BUILD_NUM")
			h.JobId = os.Getenv("CIRCLE_NODE_INDEX")
			//slug="${CIRCLE_REPOSITORY_URL##*:}"
			h.Slug = strings.Split(os.Getenv("CIRCLE_REPOSITORY_URL"), ":")[1]
			if os.Getenv("CIRCLE_PROJECT_REPONAME") != "" {
				h.Slug = os.Getenv("CIRCLE_PROJECT_USERNAME") + "/" + os.Getenv("CIRCLE_PROJECT_REPONAME")
			} else {
				h.Slug = strings.TrimSuffix(h.Slug, ".git")
			}
			return nil
		},
	},
	{
		Name:       "buddybuild",
		IsDetected: checkForEnvVars("BUDDYBUILD_BRANCH"),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("BUDDYBUILD_BRANCH")
			h.BuildId = fmt.Sprintf(
				"https://dashboard.buddybuild.com/public/apps/%v/build/%v",
				os.Getenv("BUDDYBUILD_APP_ID"),
				os.Getenv("BUDDYBUILD_BUILD_ID"),
			)
			return nil
		},
	},
	{
		Name:       "Bamboo",
		IsDetected: checkForEnvVars("bamboo_planRepository_revision"),
		FindInfo: func(h *protocol.Hello) error {
			h.CommitHash = os.Getenv("bamboo_planRepository_revision")
			h.BranchName = os.Getenv("bamboo_planRepository_branch")
			h.BuildId = os.Getenv("bamboo_buildNumber")
			h.BuildUrl = os.Getenv("bamboo_buildResultsUrl")
			//TODO: another remote addr
			//remote_addr="${bamboo_planRepository_repositoryUrl}"
			return nil
		},
	},
	{
		Name: "Bitrise CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":         "true",
			"BITRISE_IO": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("BITRISE_GIT_BRANCH")
			h.CommitHash = coalesce(os.Getenv("GIT_CLONE_COMMIT_HASH"), h.CommitHash)
			h.PullRequest = os.Getenv("BITRISE_PULL_REQUEST")
			h.BuildId = os.Getenv("BITRISE_BUILD_NUMBER")
			h.BuildUrl = os.Getenv("BITRISE_BUILD_URL")
			return nil
		},
	},
	{
		Name: "Semaphore CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":        "true",
			"SEMAPHORE": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("SEMAPHORE_GIT_BRANCH")
			h.CommitHash = os.Getenv("REVISION")
			h.PullRequest = os.Getenv("PULL_REQUEST_NUMBER")
			h.BuildId = os.Getenv("SEMAPHORE_WORKFLOW_NUMBER")
			h.JobId = os.Getenv("SEMAPHORE_JOB_ID")
			h.Slug = os.Getenv("SEMAPHORE_REPO_SLUG")
			return nil
		},
	},
	{
		Name: "Buildkite CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":        "true",
			"BUILDKITE": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("BUILDKITE_BRANCH")
			h.CommitHash = os.Getenv("BUILDKITE_COMMIT")
			if os.Getenv("BUILDKITE_PULL_REQUEST") != "false" {
				h.PullRequest = os.Getenv("BUILDKITE_PULL_REQUEST")
			}
			h.BuildId = os.Getenv("BUILDKITE_BUILD_NUMBER")
			h.JobId = os.Getenv("BUILDKITE_JOB_ID")
			h.Slug = os.Getenv("BUILDKITE_PROJECT_SLUG")
			h.BuildUrl = os.Getenv("BUILDKITE_BUILD_URL")
			h.Tag = os.Getenv("BUILDKITE_TAG")
			return nil
		},
	},
	{
		Name: "Heroku CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":                     "true",
			"HEROKU_TEST_RUN_BRANCH": "*",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("HEROKU_TEST_RUN_BRANCH")
			h.CommitHash = os.Getenv("HEROKU_TEST_RUN_COMMIT_VERSION")
			h.BuildId = os.Getenv("HEROKU_TEST_RUN_ID")
			return nil
		},
	},
	{
		Name: "Appveyor",
		IsDetected: func() bool {
			return (os.Getenv("CI") == "true" || os.Getenv("CI") == "True") &&
				(os.Getenv("APPVEYOR") == "true" || os.Getenv("APPVEYOR") == "True")
		},
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("APPVEYOR_REPO_BRANCH")
			h.CommitHash = os.Getenv("APPVEYOR_REPO_COMMIT")
			//TODO: why does codeconv urlencode this?
			h.BuildId = os.Getenv("APPVEYOR_JOB_ID")
			h.PullRequest = os.Getenv("APPVEYOR_PULL_REQUEST_NUMBER")
			h.JobId = fmt.Sprintf(
				"%v/%v/%v",
				os.Getenv("APPVEYOR_ACCOUNT_NAME"),
				os.Getenv("APPVEYOR_PROJECT_SLUG"),
				os.Getenv("APPVEYOR_BUILD_VERSION"),
			)
			h.Slug = os.Getenv("APPVEYOR_REPO_NAME")
			h.BuildUrl = fmt.Sprintf(
				"%v/project/%v/builds/%v/job/%v",
				os.Getenv("APPVEYOR_URL"),
				os.Getenv("APPVEYOR_REPO_NAME"),
				os.Getenv("APPVEYOR_BUILD_ID"),
				os.Getenv("APPVEYOR_JOB_ID"),
			)
			return nil
		},
	},
	{
		Name: "Wercker CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":                 "true",
			"WERCKER_GIT_BRANCH": "*",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("WERCKER_GIT_BRANCH")
			h.CommitHash = os.Getenv("WERCKER_GIT_COMMIT")
			h.BuildId = os.Getenv("WERCKER_MAIN_PIPELINE_STARTED")
			h.Slug = os.Getenv("WERCKER_GIT_OWNER") + "/" + os.Getenv("WERCKER_GIT_REPOSITORY")
			return nil
		},
	},
	{
		Name: "Magnum CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":     "true",
			"MAGNUM": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("CI_BRANCH")
			h.CommitHash = os.Getenv("CI_COMMIT")
			h.BuildId = os.Getenv("CI_BUILD_NUMBER")
			return nil
		},
	},
	{
		Name: "Shippable CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"SHIPPABLE": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = coalesceEnv("HEAD_BRANCH", "BRANCH")
			h.CommitHash = os.Getenv("COMMIT")
			h.BuildId = os.Getenv("BUILD_NUMBER")
			h.BuildUrl = os.Getenv("BUILD_URL")
			h.PullRequest = os.Getenv("PULL_REQUEST")
			h.Slug = os.Getenv("REPO_FULL_NAME")
			return nil
		},
	},
	{
		Name: "Solano CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"TDDIUM": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("TDDIUM_CURRENT_BRANCH")
			h.CommitHash = os.Getenv("TDDIUM_CURRENT_COMMIT")
			h.BuildId = os.Getenv("TDDIUM_TID")
			h.PullRequest = os.Getenv("TDDIUM_PR_ID")
			return nil
		},
	},
	{
		Name: "Greenhouse CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"GREENHOUSE": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("GREENHOUSE_BRANCH")
			h.CommitHash = os.Getenv("GREENHOUSE_COMMIT")
			h.BuildId = os.Getenv("GREENHOUSE_BUILD_NUMBER")
			h.PullRequest = os.Getenv("GREENHOUSE_PULL_REQUEST")
			h.BuildUrl = os.Getenv("$GREENHOUSE_BUILD_URL")
			return nil
		},
	},
	{
		Name:       "Gitlab CI",
		IsDetected: checkForEnvVars("GITLAB_CI"),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = coalesceEnv("CI_BUILD_REF_NAME", "CI_COMMIT_REF_NAME")
			h.CommitHash = coalesceEnv("CI_BUILD_REF", "CI_COMMIT_SHA")
			h.BuildId = coalesceEnv("CI_BUILD_ID", "CI_JOB_ID")
			h.Slug = os.Getenv("CI_PROJECT_PATH")
			//TODO: ???
			//remote_addr="${CI_BUILD_REPO:-$CI_REPOSITORY_URL}"
			return nil
		},
	},
	{
		Name:       "Github Actions",
		IsDetected: checkForEnvVars("GITHUB_ACTIONS"),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("GITHUB_REF") + "refs/heads/"
			if os.Getenv("GITHUB_HEAD_REF") != "" {
				//# "PR refs are in the format: refs/pull/7/merge"
				h.PullRequest = strings.TrimPrefix(
					strings.TrimSuffix(os.Getenv("GITHUB_REF"), "/merge"),
					"refs/pull/",
				)
				h.BranchName = os.Getenv("GITHUB_HEAD_REF")
			}
			h.CommitHash = os.Getenv("GITHUB_SHA")
			h.BuildId = os.Getenv("GITHUB_RUN_ID")
			h.Slug = os.Getenv("GITHUB_REPOSITORY")
			h.BuildUrl = fmt.Sprintf(
				"http://github.com/%v/actions/runs/%v",
				os.Getenv("GITHUB_REPOSITORY"),
				os.Getenv("GITHUB_RUN_ID"),
			)
			// TODO: "actions/checkout runs in detached HEAD"
			// need to fix commit SHA
			//mc=
			//if [ -n "$pr" ] && [ "$pr" != false ];
			//then
			//mc=$(git show --no-patch --format="%P" 2>/dev/null || echo "")
			//fi
			//if [[ "$mc" =~ ^[a-z0-9]{40}[[:space:]][a-z0-9]{40}$ ]];
			//then
			//say "    Fixing merge commit SHA"
			//commit=$(echo "$mc" | cut -d' ' -f2)
			//fi
			return nil
		},
	},
	{
		Name:       "Azure Pipelines",
		IsDetected: checkForEnvVars("SYSTEM_TEAMFOUNDATIONSERVERURI"),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("BUILD_SOURCEBRANCHNAME")
			h.CommitHash = os.Getenv("BUILD_SOURCEVERSION")
			h.BuildId = os.Getenv("BUILD_BUILDNUMBER")
			h.PullRequest = coalesceEnv("SYSTEM_PULLREQUEST_PULLREQUESTNUMBER", "SYSTEM_PULLREQUEST_PULLREQUESTID")
			h.JobId = os.Getenv("BUILD_BUILDID")
			h.BuildUrl = fmt.Sprintf(
				"%v%v/_build/results?buildId=%v",
				os.Getenv("SYSTEM_TEAMFOUNDATIONSERVERURI"),
				os.Getenv("SYSTEM_TEAMPROJECT"),
				os.Getenv("BUILD_BUILDID"),
			)
			//TODO: ???
			//project="${SYSTEM_TEAMPROJECT}"
			//server_uri="${SYSTEM_TEAMFOUNDATIONSERVERURI}"
			return nil
		},
	},
	{
		Name: "Bitbucket",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":                     "true",
			"BITBUCKET_BUILD_NUMBER": "*",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("BITBUCKET_BRANCH")
			h.CommitHash = os.Getenv("BITBUCKET_COMMIT")
			h.BuildId = os.Getenv("BITBUCKET_BUILD_NUMBER")
			h.Slug = os.Getenv("BITBUCKET_REPO_OWNER") + "/" + os.Getenv("BITBUCKET_REPO_SLUG")
			h.JobId = os.Getenv("BITBUCKET_BUILD_NUMBER")
			h.PullRequest = os.Getenv("BITBUCKET_PR_ID")
			//# See https://jira.atlassian.com/browse/BCLOUD-19393
			if h.CommitHash == "12" {
				output, err := exec.Command("git", "rev-parse", "$BITBUCKET_COMMIT").Output()
				if err != nil {
					return errors.Wrap(err, "get commit hash for bitbucket")
				}
				h.CommitHash = string(output)
			}
			return nil
		},
	},
	{
		Name: "Buddy CI",
		IsDetected: checkForEnvVarsMap(map[string]string{
			"CI":    "true",
			"BUDDY": "true",
		}),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("BUDDY_EXECUTION_BRANCH")
			h.CommitHash = os.Getenv("BUDDY_EXECUTION_REVISION")
			h.BuildId = os.Getenv("BUDDY_EXECUTION_ID")
			h.BuildUrl = os.Getenv("BUDDY_EXECUTION_URL")
			h.Slug = os.Getenv("BUDDY_REPO_SLUG")
			h.PullRequest = os.Getenv("BUDDY_EXECUTION_PULL_REQUEST_NO")
			h.Tag = os.Getenv("BUDDY_EXECUTION_TAG")
			return nil
		},
	},
	{
		Name:       "Cirrus CI",
		IsDetected: checkForEnvVars("$CIRRUS_CI"),
		FindInfo: func(h *protocol.Hello) error {
			h.BranchName = os.Getenv("CIRRUS_BRANCH")
			h.CommitHash = os.Getenv("CIRRUS_CHANGE_IN_REPO")
			h.BuildId = os.Getenv("CIRRUS_TASK_ID")
			h.Slug = os.Getenv("CIRRUS_REPO_FULL_NAME")
			h.PullRequest = os.Getenv("CIRRUS_PR")
			h.JobId = os.Getenv("CIRRUS_TASK_NAME")
			return nil
		},
	},
}

/* Populates a Hello message with metadata about the pipeline's author */
func populateAuthorInfo(h *protocol.Hello) error {
	// TODO: git might not be available
	// TODO: how to get author avatar?
	currentBranchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := currentBranchCmd.Output()
	if err != nil {
		return err
	}
	currentBranch := strings.TrimSpace(string(output))
	//git for-each-ref --count=1 refs/heads/
	authorDetailsCmd := exec.Command("git", "for-each-ref", "--count=1", "refs/heads/"+currentBranch,
		"--format=%(authorname) %(authoremail)")
	output, err = authorDetailsCmd.Output()
	if err != nil {
		return err
	}
	parts := strings.Split(strings.TrimSpace(string(output)), "<")
	// Author Name <authoremail@example.com>
	h.AuthorName = strings.TrimSpace(parts[0])
	h.AuthorEmail = strings.TrimSuffix(parts[1], ">")
	if h.AuthorEmail != "" {
		parts = strings.Split(h.AuthorEmail, "@")
		if len(parts) == 2 {
			h.AuthorEmailDomain = parts[1]
		}
	}
	return nil
}

/*
Populates a Hello message with metadata about the pipeline

Some of this may later be redacted, based on privacy settings, before the message is sent.*/
func populatePipelineInfo(h *protocol.Hello) []error {
	errs := []error{}
	err := populateAuthorInfo(h)
	if err != nil {
		errs = append(errs, errors.Wrap(err, "author info"))
	}
	// based on codecov bash uploader, extended for more git info
	// https://github.com/codecov/codecov-bash/blob/master/codecov
	h.CommitHash = os.Getenv("VCS_COMMIT_ID")
	h.BranchName = os.Getenv("VCS_BRANCH_NAME")
	h.PullRequest = os.Getenv("VCS_PULL_REQUEST")
	h.Slug = os.Getenv("VCS_SLUG")
	h.Tag = os.Getenv("VCS_TAG")
	h.BuildUrl = os.Getenv("CI_BUILD_URL")
	h.BuildId = os.Getenv("CI_BUILD_ID")
	h.JobId = os.Getenv("CI_JOB_ID")
	for _, provider := range providers {
		if provider.IsDetected() {
			h.CiProvider = provider.Name
			err = provider.FindInfo(h)
			if err != nil {
				errs = append(errs, errors.Wrap(err, "provider info"))
			}
			break
		}
	}
	return errs
}

/*
Redacts certain pipeline metadata, based on a provided mask map.
Fields that appear in the map are redacted.
*/
func redactPipelineInfo(h *protocol.Hello, m map[string]bool) {
	if _, redact := m["CommitHash"]; redact {
		h.CommitHash = ""
	}
	if _, redact := m["BranchName"]; redact {
		h.BranchName = ""
	}
	if _, redact := m["PullRequest"]; redact {
		h.PullRequest = ""
	}
	if _, redact := m["Slug"]; redact {
		h.Slug = ""
	}
	if _, redact := m["Tag"]; redact {
		h.Tag = ""
	}
	if _, redact := m["BuildUrl"]; redact {
		h.BuildUrl = ""
	}
	if _, redact := m["BuildId"]; redact {
		h.BuildId = ""
	}
	if _, redact := m["JobId"]; redact {
		h.JobId = ""
	}
	if _, redact := m["AuthorName"]; redact {
		h.AuthorName = ""
	}
	if _, redact := m["AuthorAvatar"]; redact {
		h.AuthorAvatar = ""
	}
	if _, redact := m["AuthorEmail"]; redact {
		h.AuthorEmail = ""
	}
	if _, redact := m["CiProvider"]; redact {
		h.CiProvider = ""
	}
	if _, redact := m["AuthorEmailDomain"]; redact {
		h.AuthorEmailDomain = ""
	}
}
