package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
	logger "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	credentialEntities "github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
	gitInfra "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
)

// ErrNoAPIToken reports that AutoBump has no token to call the API of a repository's forge
// with: none is configured for the forge, and the credential fallback found none either.
var ErrNoAPIToken = errors.New("no API token")

// apiTokenSettings names the configuration key that holds each forge's API token. A forge
// without one is a forge AutoBump has no API client for, so the fallback is never asked for
// its token: no token, configured or found, could open a pull request there.
//
//nolint:gochecknoglobals // read-only lookup table
var apiTokenSettings = map[entities.ServiceType]string{
	entities.UNKNOWN:     "",
	entities.GITHUB:      "github_access_token",
	entities.GITLAB:      "gitlab_access_token",
	entities.AZUREDEVOPS: "azure_devops_access_token",
	entities.BITBUCKET:   "",
	entities.CODECOMMIT:  "",
	entities.CODEBERG:    "",
}

// credentialFallback is set by the application at startup via SetCredentialFallback.
var credentialFallback credentialEntities.CredentialResolver //nolint:gochecknoglobals // set once at startup

// SetCredentialFallback sets where the forge API calls look for a token when the
// configuration holds none for the forge. The application installs gitforge's default
// credential chain at startup; left nil, as it is in tests, there is no fallback, so no test
// ever runs gh or az.
func SetCredentialFallback(resolver credentialEntities.CredentialResolver) {
	credentialFallback = resolver
}

// fallbackAnswer is what the credential fallback answered to one request: the credential it
// supplied, or the error explaining why it supplied none.
type fallbackAnswer struct {
	credential credentialEntities.Credential
	err        error
}

// resolveAPIToken returns the token AutoBump calls the forge's API with: to open a pull
// request, to check whether one is already open, and to close a stale one.
//
// A token the configuration holds always wins (resolveToken): project_access_token, then the
// forge's own key. Only when there is none is ctx.CredentialFallback asked, which in
// production is gitforge's credential chain: the forge's token environment variables
// (GITHUB_TOKEN and GH_TOKEN, GITLAB_TOKEN and GL_TOKEN, AZURE_DEVOPS_EXT_PAT and
// SYSTEM_ACCESSTOKEN), then its own CLI (`gh auth token --hostname github.com`, or
// `az account get-access-token` in the organization's tenant). Someone signed in to gh or az
// has already proven who they are; without the fallback, `autobump .` pushed the release
// branch and then skipped the pull request until they exported the same token again.
//
// Discovery never gets this far: every repository it finds carries its provider's token as
// project_access_token, and ValidateProviders refuses a provider without one.
//
// The fallback's token is the operator's own, and it is fit for the API calls alone:
// gitforge's providers send it to the forge's fixed API host (api.github.com, gitlab.com,
// dev.azure.com), which is the forge that issued it. Git operations never use it; see
// collectTokens for why.
func resolveAPIToken(ctx *RepoContext, serviceType entities.ServiceType) (string, error) {
	if token := resolveToken(serviceType, ctx.GlobalConfig, ctx.ProjectConfig); token != "" {
		return token, nil
	}

	forge := serviceType.DisplayName()
	setting := apiTokenSettings[serviceType]
	if setting == "" {
		return "", fmt.Errorf("%w for %s: AutoBump has no API client for it", ErrNoAPIToken, forge)
	}
	if ctx.CredentialFallback == nil {
		return "", fmt.Errorf("%w for %s: %s is not set", ErrNoAPIToken, forge, setting)
	}

	answer := askCredentialFallback(ctx, credentialEntities.CredentialRequest{
		ServiceType:  serviceType,
		Organization: azureDevOpsOrganization(ctx.Repo, serviceType),
	})
	if answer.err != nil {
		return "", fmt.Errorf("%w for %s: %s is not set, and %w", ErrNoAPIToken, forge, setting, answer.err)
	}
	return answer.credential.Token, nil
}

// askCredentialFallback returns ctx.CredentialFallback's answer to a request, asking only the
// first time and keeping the answer, a failure included, for the rest of the repository's
// release. gh answers at once, but az may look up the organization's tenant and talk to
// Microsoft Entra first, and asking again would only repeat the wait and the failure.
//
// The answer is keyed by the whole request, so a token found for one forge never answers a
// call to another. The INFO line naming where the token came from is logged when it is first
// found; the token itself is never logged.
func askCredentialFallback(ctx *RepoContext, request credentialEntities.CredentialRequest) fallbackAnswer {
	if answer, asked := ctx.fallbackAnswers[request]; asked {
		return answer
	}

	credential, err := ctx.CredentialFallback.Resolve(context.Background(), request)
	if err == nil && credential.Token == "" {
		err = fmt.Errorf("%w: the credential fallback returned an empty token", credentialEntities.ErrNoCredential)
	}

	answer := fallbackAnswer{credential: credential, err: err}
	if err != nil {
		// Whatever a failed lookup handed back is not a token to keep.
		answer.credential = credentialEntities.Credential{}
	} else {
		logger.Infof(
			"No %s configured; calling the %s API with the token from the %s",
			apiTokenSettings[request.ServiceType], request.ServiceType.DisplayName(), credential.Source,
		)
	}

	if ctx.fallbackAnswers == nil {
		ctx.fallbackAnswers = make(map[credentialEntities.CredentialRequest]fallbackAnswer)
	}
	ctx.fallbackAnswers[request] = answer
	return answer
}

// azureDevOpsOrganization returns the Azure DevOps organization origin points into, so the
// Azure CLI issues its token in that organization's Microsoft Entra tenant: the tenant of the
// signed-in account's default subscription can be another one, whose token the organization
// refuses. It returns "", which keeps the CLI's default tenant, for every other forge and for
// an origin gitforge cannot parse.
func azureDevOpsOrganization(repo *git.Repository, serviceType entities.ServiceType) string {
	if serviceType != entities.AZUREDEVOPS {
		return ""
	}

	parsed, err := gitInfra.ParseRemoteURL(originRemoteURL(repo))
	if err != nil || parsed.ServiceType != entities.AZUREDEVOPS {
		return ""
	}
	return parsed.Organization
}
