package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	log "github.com/sirupsen/logrus"
)

// detectLanguage detects the language of a project by looking at the files in the project
func detectLanguage(globalConfig *GlobalConfig, cwd string) (string, error) {
	log.Info("Detecting project language")

	absPath, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}

	// check the project type by special files
	for language, config := range globalConfig.LanguagesConfig {
		for _, pattern := range config.SpecialPatterns {
			matches, _ := filepath.Glob(filepath.Join(absPath, pattern))
			if len(matches) > 0 {
				log.Infof(
					"Project language detected as %s via file pattern '%s'",
					language,
					pattern,
				)
				return language, nil
			}
		}
	}

	// check the project type by file extensions
	var detected string
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

	if detected != "" {
		log.Infof("Project language detected as '%s' via file extension", detected)
		return detected, nil
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
	if strings.HasPrefix(projectConfig.Path, "https://") ||
		strings.HasPrefix(projectConfig.Path, "git@") {
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

		service := getServiceTypeByURL(projectConfig.Path)
		auth, err := getAuthenticationMethod(service, globalGitConfig.Raw.Section("user").Option("name"), globalConfig, projectConfig)
		if err != nil {
			return err
		}

		cloneOptions.Auth = auth
		_, err = git.PlainClone(tmpDir, false, cloneOptions)
		if err != nil {
			return err
		}
		log.Infof("Successfully cloned %s", projectConfig.Path)
		projectConfig.Path = tmpDir
	}

	projectPath := projectConfig.Path
	changelogPath := filepath.Join(projectPath, "CHANGELOG.md")

	err = createChangelogIfNotExists(changelogPath)
	if err != nil {
		return err
	}

	bumpEmpty, err := isChangelogUnreleasedEmpty(changelogPath)
	if err != nil {
		return err
	}
	if bumpEmpty {
		log.Infof("Bump is empty, skipping project %s", projectConfig.Name)
		return nil
	}

	// detect the project language if not manually set
	if projectConfig.Language == "" {
		projectLanguage, err := detectLanguage(globalConfig, projectConfig.Path)
		if err != nil {
			return err
		}
		projectConfig.Language = projectLanguage
	}

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

	nextVersion, err := getNextVersion(changelogPath)
	if err != nil {
		return err
	}

	branchName := fmt.Sprintf("chore/bump-%s", nextVersion.String())

	// check if branch already exists
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
	version, err := updateChangelogFile(changelogPath)
	if err != nil {
		log.Errorf("No version found in CHANGELOG.md for project at %s\n", projectConfig.Path)
		return err
	}

	projectConfig.NewVersion = version.String()
	log.Infof("Updating version to %s", projectConfig.NewVersion)
	err = updateVersion(globalConfig, projectConfig)
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

	// TODO: we should create methods to return these sections without repeating code
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

		// TODO: we should create methods to return these sections without repeating code
		gpgKeyId := cfg.Raw.Section("user").Option("signingkey")
		if gpgKeyId == "" {
			gpgKeyId = globalGitConfig.Raw.Section("user").Option("signingkey")
		}
		signKey, err = getGpgKey(gpgKeyId, globalConfig.GpgKeyPath)
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
	} else if serviceType == "AzureDevOps" {
		err = createAzureDevOpsPullRequest(
			globalConfig,
			projectConfig,
			repo,
			branchName,
			projectConfig.NewVersion,
		)
		if err != nil {
			return err
		}
	} else {
		log.Warnf("Service type '%s' not supported yet...", serviceType)
	}

	err = checkoutBranch(w, "main")
	if err != nil {
		return checkoutBranch(w, "master")
	}

	log.Infof("Successfully processed project '%s'", projectConfig.Name)

	return nil
}

// iterateProjects iterates over the projects and processes them using the processRepo function
func iterateProjects(globalConfig *GlobalConfig) {
	for _, project := range globalConfig.Projects {

		// verify if the project path exists
		if _, err := os.Stat(project.Path); os.IsNotExist(err) {
			if !strings.HasPrefix(project.Path, "https://") &&
				!strings.HasPrefix(project.Path, "git@") {
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
