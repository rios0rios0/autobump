package commands

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	logger "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	gitInfra "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// closePullRequestTimeout caps a single close call against the forge API.
const closePullRequestTimeout = 30 * time.Second

// bumpBranchVersionRegex matches every version createBumpBranch appends to the prefix:
// X.Y.Z, the only shape getNextVersion produces (every bump goes through IncMajor,
// IncMinor or IncPatch, which drop prerelease and build metadata), and the fork forms
// X.Y.Z.N and X.Y.Z-N.
var bumpBranchVersionRegex = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[.\-]\d+)?$`)

// filterStaleBumpBranches returns the branches AutoBump owns and may safely remove: the
// bump prefix followed by a version the bumper names a branch after, except the
// repository's default branch.
//
// Ownership is decided by the whole name, not by the prefix. createBumpBranch only ever
// appends a version to the prefix, so a branch with anything else after it -- nothing at
// all, `chore/bump-runner-images`, `chore/bump-network-module-1.15.0` -- was named by
// somebody else, and people do name their own branches that way. A prefix match alone
// deleted those branches and closed their pull requests.
//
// Merge status is deliberately ignored. A bump branch is disposable by construction —
// the bumper recreates whichever one it needs on the next run — so an unmerged bump
// branch is a leftover from a release nobody completed, not work worth preserving.
// The result is sorted so cleanup order (and its log output) is deterministic.
//
// entities.ValidateBumpBranchPrefix rejects the prefixes that would make this reach far —
// this is the second lock on the same door, at the point where the deleting happens.
func filterStaleBumpBranches(branches []string, prefix, defaultBranch string) []string {
	stale := make([]string, 0, len(branches))
	for _, branch := range branches {
		if branch == defaultBranch {
			continue
		}
		version, found := strings.CutPrefix(branch, prefix)
		if !found || !bumpBranchVersionRegex.MatchString(version) {
			continue
		}
		stale = append(stale, branch)
	}
	slices.Sort(stale)
	return stale
}

// cleanupStaleBumpBranches deletes every stale bump branch from the remote and closes the
// pull request attached to it, so repeated unattended runs cannot pile up abandoned
// release branches waiting on a review that never comes.
//
// It is best-effort by design: every failure is logged and the remaining branches are
// still processed. Cleanup is housekeeping, so it must never abort the release it runs
// ahead of — a repository that failed to clean is far better than one that failed to
// bump.
func cleanupStaleBumpBranches(ctx *RepoContext) {
	serviceType, err := gitOps.GetRemoteServiceType(ctx.Repo)
	if err != nil {
		logger.Warnf("Could not determine the service type, skipping stale branch cleanup: %v", err)
		return
	}

	username := ctx.GlobalGitConfig.Raw.Section("user").Option("name")
	authMethods := collectAuthMethods(serviceType, username, ctx.GlobalConfig, ctx.ProjectConfig)

	branches, err := gitInfra.ListRemoteBranches(ctx.Repo, authMethods)
	if err != nil {
		logger.Warnf("Could not list the remote branches, skipping stale branch cleanup: %v", err)
		return
	}

	prefix := entities.ResolveBumpBranchPrefix(ctx.GlobalConfig)
	defaultBranch := resolveDefaultBranch(ctx.Repo)

	stale := filterStaleBumpBranches(branches, prefix, defaultBranch)
	if len(stale) == 0 {
		logger.Infof("No stale '%s*' branches found to clean up", prefix)
		return
	}

	logger.Infof("Cleaning up %d stale '%s*' branch(es)", len(stale), prefix)

	actions := staleBranchActions{
		deleteRemote: func(branch string) error {
			return gitInfra.DeleteRemoteBranch(ctx.Repo, branch, authMethods)
		},
		deleteLocal: func(branch string) error {
			return gitInfra.DeleteLocalBranch(ctx.Repo, branch)
		},
	}
	if provider, forgeRepo := resolveCleanupProvider(ctx, serviceType, defaultBranch); provider != nil {
		actions.closePullRequest = func(branch string) (bool, error) {
			return closeStalePullRequest(provider, forgeRepo, branch)
		}
	}

	for _, branch := range stale {
		removeStaleBumpBranch(branch, actions)
	}
}

// staleBranchActions are the side effects of removing one stale bump branch. They are
// handed to removeStaleBumpBranch rather than called from it, so the order it runs them in
// can be tested without a forge, or a remote that refuses a deletion.
type staleBranchActions struct {
	deleteRemote func(branch string) error
	deleteLocal  func(branch string) error
	// closePullRequest is nil when there is no provider to close pull requests with.
	closePullRequest func(branch string) (bool, error)
}

// removeStaleBumpBranch deletes one stale bump branch, then closes the pull request opened
// from it.
//
// The branch goes first, because closing its pull request is only worth anything once the
// branch is gone: it makes room for the fresh branch and pull request the bumper is about
// to create. A forge can refuse the deletion -- Azure DevOps lets an identity delete only
// the branches it created unless it holds the Force push permission, so every bump branch
// someone else pushed is refused -- and closing first turned each refusal into an
// abandoned pull request plus a duplicate, opened by the bumper from the very branch that
// could not be removed, on every run. Refused now, the branch and its pull request are left
// as they were, and the bumper finds that pull request still open and leaves it to its
// reviewers.
//
// The order costs the closing nothing: every provider looks the pull request up by the
// name of its source branch, which outlives the branch, and GitHub closes it by itself the
// moment its head branch is deleted. Without a provider the bumper never opened a pull
// request in the first place, so the branch is all there is to remove.
func removeStaleBumpBranch(branch string, actions staleBranchActions) {
	if err := actions.deleteRemote(branch); err != nil {
		logger.Warnf(
			"Could not delete the stale branch '%s', leaving it and its pull request in place: %v",
			branch, err,
		)
		return
	}

	// The local branch has to go too. CheckBranchExists reports a branch as existing
	// when it finds it either locally or remotely, so a leftover local reference would
	// convince the bumper the branch it just deleted is still there, and it would skip
	// recreating it.
	if err := actions.deleteLocal(branch); err != nil {
		logger.Warnf("Deleted the remote branch '%s' but could not delete it locally: %v", branch, err)
	}
	logger.Infof("Deleted the stale branch '%s'", branch)

	if actions.closePullRequest == nil {
		return
	}
	closed, err := actions.closePullRequest(branch)
	if err != nil {
		logger.Warnf(
			"Deleted the stale branch '%s' but could not close its pull request, "+
				"which is left without a source branch; close it by hand: %v",
			branch, err,
		)
		return
	}
	if closed {
		logger.Infof("Closed the pull request for the stale branch '%s'", branch)
	}
}

// resolveCleanupProvider returns the forge provider used to close stale pull requests
// along with the repository it addresses. It returns a nil provider when no token is
// configured or the remote cannot be resolved; cleanup then degrades to deleting
// branches only, which is still worth doing.
func resolveCleanupProvider(
	ctx *RepoContext,
	serviceType entities.ServiceType,
	defaultBranch string,
) (globalEntities.ForgeProvider, globalEntities.Repository) {
	token := resolveToken(serviceType, ctx.GlobalConfig, ctx.ProjectConfig)
	if token == "" {
		logger.Warnf(
			"No token found for service type '%v', stale pull requests will not be closed",
			serviceType,
		)
		return nil, globalEntities.Repository{}
	}

	provider, err := getForgeProvider(serviceType, token)
	if err != nil {
		logger.Warnf(
			"Service type '%v' does not support closing pull requests: %v",
			serviceType, err,
		)
		return nil, globalEntities.Repository{}
	}

	remoteURL, err := gitInfra.GetRemoteRepoURL(ctx.Repo)
	if err != nil {
		logger.Warnf("Could not resolve the remote URL, stale pull requests will not be closed: %v", err)
		return nil, globalEntities.Repository{}
	}

	return provider, buildGitforgeRepo(remoteURL, defaultBranch)
}

// closeStalePullRequest closes the pull request opened from the given branch, if one is
// still open, and reports whether it closed one. Finding no open pull request is not a
// failure.
func closeStalePullRequest(
	provider globalEntities.ForgeProvider,
	forgeRepo globalEntities.Repository,
	branch string,
) (bool, error) {
	// Bounded so an unresponsive provider cannot hang the release behind housekeeping.
	// Cleanup runs before the bump and walks every stale branch, so without a deadline a
	// single hung call would stall the whole run rather than degrading to best-effort.
	ctx, cancel := context.WithTimeout(context.Background(), closePullRequestTimeout)
	defer cancel()

	return provider.ClosePullRequest(ctx, forgeRepo, branch)
}
