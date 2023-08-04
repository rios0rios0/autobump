package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
)

// detectLanguage detects the language of a project by looking at the files in the project
func detectLanguage(globalConfig *GlobalConfig, cwd string) (string, error) {
	var detected string

	absPath, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}

	// Check project type by special files
	for language, config := range globalConfig.LanguagesConfig {
		for _, pattern := range config.SpecialPatterns {
			_, err := os.Stat(filepath.Join(absPath, pattern))
			if !os.IsNotExist(err) {
				return language, nil
			}
		}
	}

	// Check project type by file extensions
	err = filepath.Walk(absPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if detected != "" {
			return filepath.SkipDir
		}

		for language, config := range globalConfig.LanguagesConfig {
			for _, ext := range config.Extensions {
				if strings.HasSuffix(info.Name(), "."+ext) {
					detected = language
					return filepath.SkipDir
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return "", errors.New("project language not recognized")
}

// processRepo:
// - clones the repository if it is a remote repository
// - creates the chore/bump branch
// - updates the CHANGELOG.md file
// - updates the version file
// - commits the changes
// - pushes the branch to the remote repository
// - creates a new merge request on GitLab
func processRepo(globalConfig *GlobalConfig, projectConfig *ProjectConfig) error {
	globalGitConfig, err := getGlobalGitConfig()
	if err != nil {
		return err
	}

	// check if project.Path starts with https:// or git@
	// if these prefixes exist, it means the project is a remote repository and should be cloned
	if strings.HasPrefix(projectConfig.Path, "https://") || strings.HasPrefix(projectConfig.Path, "git@") {
		tmpDir, err := os.MkdirTemp("", "autobump-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		log.Infof("Cloning %s into %s", projectConfig.Path, tmpDir)
		cloneOptions := &git.CloneOptions{
			URL:   projectConfig.Path,
			Depth: 1,
		}

		// authenticate with CI job token if running in a GitLab CI pipeline
		if projectConfig.ProjectAccessToken != "" {
			log.Infof("Using project access token to authenticate")
			cloneOptions.Auth = &http.BasicAuth{
				Username: "oauth2",
				Password: projectConfig.ProjectAccessToken,
			}
		} else if globalConfig.GitLabCIJobToken != "" {
			log.Infof("Using GitLab CI job token to authenticate")
			cloneOptions.Auth = &http.BasicAuth{
				Username: "gitlab-ci-token",
				Password: globalConfig.GitLabCIJobToken,
			}
		} else {
			log.Infof("Using GitLab access token to authenticate")
			cloneOptions.Auth = &http.BasicAuth{
				Username: globalGitConfig.Raw.Section("user").Option("name"),
				Password: globalConfig.GitLabAccessToken,
			}
		}

		_, err = git.PlainClone(tmpDir, false, cloneOptions)
		if err != nil {
			return err
		}
		log.Infof("Successfully cloned %s", projectConfig.Path)
		projectConfig.Path = tmpDir
	}

	// detect the project language if not manually set
	if projectConfig.Language == "" {
		projectLanguage, err := detectLanguage(globalConfig, projectConfig.Path)
		if err != nil {
			return err
		}
		projectConfig.Language = projectLanguage
	}

	projectPath := projectConfig.Path
	repo, err := openRepo(projectPath)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	head, err := repo.Head()
	if err != nil {
		return err
	}

	branchName := "chore/bump"
	branchExists, err := checkBranchExists(repo, branchName)
	if err != nil {
		return err
	}
	if branchExists {
		return fmt.Errorf("branch %s already exists", branchName)
	}

	err = createAndSwitchBranch(repo, w, branchName, head.Hash())
	if err != nil {
		return err
	}

	log.Info("Updating CHANGELOG.md file")
	changelogPath := filepath.Join(projectPath, "CHANGELOG.md")
	version, err := updateChangelogFile(changelogPath)
	if err != nil {
		log.Errorf("No version found in CHANGELOG.md for project at %s\n", projectConfig.Path)
		return err
	}

	projectConfig.NewVersion = version.String()
	log.Infof("Updating version to %s", projectConfig.NewVersion)
	err = updateVersion(projectPath, globalConfig, projectConfig)
	if err != nil {
		return err
	}

	versionFiles, err := getVersionFiles(globalConfig, projectConfig)
	if err != nil {
		return err
	}

	// get version file relative path
	for _, versionFile := range versionFiles {
		versionFileRelativePath, err := filepath.Rel(projectPath, versionFile.Path)
		if _, err := os.Stat(versionFile.Path); os.IsNotExist(err) {
			continue
		}

		log.Infof("Adding version file %s", versionFileRelativePath)
		_, err = w.Add(versionFileRelativePath)
		if err != nil {
			return err
		}
	}

	changelogRelativePath, err := filepath.Rel(projectPath, changelogPath)
	if err != nil {
		return err
	}
	_, err = w.Add(changelogRelativePath)
	if err != nil {
		return err
	}

	cfg, err := repo.Config()
	if err != nil {
		return err
	}

	gpgSign := cfg.Raw.Section("commit").Option("gpgsign")
	if gpgSign == "" {
		gpgSign = globalGitConfig.Raw.Section("commit").Option("gpgsign")
	}

	gpgFormat := cfg.Raw.Section("gpg").Option("format")
	if gpgFormat == "" {
		gpgFormat = globalGitConfig.Raw.Section("gpg").Option("format")
	}

	var signKey *openpgp.Entity
	if gpgSign == "true" && gpgFormat != "ssh" {
		log.Info("Signing commit with GPG key")
		signKey, err = getGpgKey(globalConfig.GpgKeyPath)
	}

	if err != nil {
		return err
	}

	commitMessage := "chore(bump): bumped version to " + projectConfig.NewVersion
	commit, err := commitChanges(
		w,
		commitMessage,
		signKey,
		globalGitConfig.Raw.Section("user").Option("name"),
		globalGitConfig.Raw.Section("user").Option("email"),
	)
	if err != nil {
		return err
	}

	_, err = repo.CommitObject(commit)
	if err != nil {
		return err
	}

	refSpec := config.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName)

	remoteCfg, err := repo.Remote("origin")
	if err != nil {
		return err
	}

	remoteURL := remoteCfg.Config().URLs[0]
	if strings.HasPrefix(remoteURL, "git@") {
		err = pushChangesSsh(repo, refSpec)
	} else if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		err = pushChangesHttps(repo, cfg, refSpec, globalConfig, projectConfig)
	}

	if err != nil {
		if err.Error() == "object not found" {
			log.Error("Got error object not found (remote branch already exists?)")
		}
		return err
	}

	serviceType, err := getRemoteServiceType(repo)
	if err != nil {
		return err
	}

	if serviceType == "GitLab" {
		err = createGitLabMergeRequest(
			globalConfig,
			projectConfig,
			repo,
			branchName,
			projectConfig.NewVersion,
		)
		if err != nil {
			return err
		}
	}

	log.Infof("Successfully processed project %s", projectConfig.Name)

	return nil
}

// iterateProjects iterates over the projects and processes them using the processRepo function
func iterateProjects(globalConfig *GlobalConfig) {
	for _, project := range globalConfig.Projects {

		// verify if the project path exists
		if _, err := os.Stat(project.Path); os.IsNotExist(err) {
			if !strings.HasPrefix(project.Path, "https://") && !strings.HasPrefix(project.Path, "git@") {
				log.Errorf("Project path does not exist: %s\n", project.Path)
				log.Warn("Skipping project")
				continue
			}
		}

		err := processRepo(globalConfig, &project)
		if err != nil {
			log.Errorf("Error processing project at %s: %v\n", project.Path, err)
		}
	}
}
