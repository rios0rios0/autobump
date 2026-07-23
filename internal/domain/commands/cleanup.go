package commands

import (
	"context"
	"slices"
	"strings"
	"time"

	logger "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// closePullRequestTimeout caps a single close call against the forge API.
const closePullRequestTimeout = 30 * time.Second

// filterStaleBumpBranches returns the branches AutoBump owns and may safely remove:
// every branch carrying the bump prefix, except the repository's default branch.
//
// Merge status is deliberately ignored. A bump branch is disposable by construction —
// the bumper recreates whichever one it needs on the next run — so an unmerged bump
// branch is a leftover from a release nobody completed, not work worth preserving.
// The result is sorted so cleanup order (and its log output) is deterministic.
func filterStaleBumpBranches(branches []string, prefix, defaultBranch string) []string {
	stale := make([]string, 0, len(branches))
	for _, branch := range branches {
		if branch == defaultBranch {
			continue
		}
		if !strings.HasPrefix(branch, prefix) {
			continue
		}
		stale = append(stale, branch)
	}
	slices.Sort(stale)
	return stale
}

// cleanupStaleBumpBranches closes the pull request attached to every stale bump branch
// and deletes the branch from the remote, so repeated unattended runs cannot pile up
// abandoned release branches waiting on a review that never comes.
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
	provider, forgeRepo := resolveCleanupProvider(ctx, serviceType, defaultBranch)

	for _, branch := range stale {
		// A branch whose pull request could not be closed is left alone. Deleting it
		// would strand an open pull request whose source branch no longer exists, and
		// because cleanup only looks at branches that still exist, no later run would
		// see it to try closing it again. Keeping it makes the pair retryable.
		//
		// A missing provider is different: without a token the bumper never opened a
		// pull request in the first place, so there is nothing to strand and the branch
		// is still worth removing.
		if provider != nil && !closeStalePullRequest(provider, forgeRepo, branch) {
			continue
		}

		if deleteErr := gitInfra.DeleteRemoteBranch(ctx.Repo, branch, authMethods); deleteErr != nil {
			logger.Warnf("Could not delete the stale branch '%s': %v", branch, deleteErr)
			continue
		}

		// The local branch has to go too. CheckBranchExists reports a branch as existing
		// when it finds it either locally or remotely, so a leftover local reference would
		// convince the bumper the branch it just deleted is still there, and it would skip
		// recreating it.
		if localErr := gitInfra.DeleteLocalBranch(ctx.Repo, branch); localErr != nil {
			logger.Warnf(
				"Deleted the remote branch '%s' but could not delete it locally: %v",
				branch, localErr,
			)
		}

		logger.Infof("Deleted the stale branch '%s'", branch)
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
// still open. It runs before the branch is deleted, because a provider cannot reliably
// resolve a pull request from a source branch that no longer exists.
//
// It reports whether the branch is safe to delete: false means the pull request could not
// be closed, so the branch must stay for a later run to retry. Finding no open pull
// request is a success, not a failure.
func closeStalePullRequest(
	provider globalEntities.ForgeProvider,
	forgeRepo globalEntities.Repository,
	branch string,
) bool {
	// Bounded so an unresponsive provider cannot hang the release behind housekeeping.
	// Cleanup runs before the bump and walks every stale branch, so without a deadline a
	// single hung call would stall the whole run rather than degrading to best-effort.
	ctx, cancel := context.WithTimeout(context.Background(), closePullRequestTimeout)
	defer cancel()

	closed, err := provider.ClosePullRequest(ctx, forgeRepo, branch)
	if err != nil {
		logger.Warnf(
			"Could not close the pull request for the branch '%s', "+
				"keeping the branch so a later run can retry: %v",
			branch, err,
		)
		return false
	}
	if closed {
		logger.Infof("Closed the pull request for the stale branch '%s'", branch)
	}
	return true
}
