package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

func getGpgKey(gpgKeyPath string) (*openpgp.Entity, error) {
	privateKeyFile, err := os.Open(gpgKeyPath)
	if err != nil {
		log.Error("Failed to open private key file:", err)
	}
	entityList, err := openpgp.ReadArmoredKeyRing(privateKeyFile)
	if err != nil {
		log.Error("Failed to read private key file:", err)
	}

	fmt.Print("Enter the passphrase for your GPG key: ")
	passphrase, err := terminal.ReadPassword(0)
	if err != nil {
		return nil, err
	}
	fmt.Println()

	entity := entityList[0]
	err = entity.PrivateKey.Decrypt([]byte(passphrase))
	if err != nil {
		log.Error("Failed to decrypt GPG key:", err)
		return nil, err
	}

	log.Info("Successfully decrypted GPG key")
	return entity, nil
}

func processRepo(globalConfig *GlobalConfig, projectsConfig *ProjectsConfig) error {
	// check if project.Path starts with https:// or git@
	if strings.HasPrefix(projectsConfig.Path, "https://") || strings.HasPrefix(projectsConfig.Path, "git@") {
		tmpDir, err := os.MkdirTemp("", "autobump-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		log.Infof("Cloning %s into %s", projectsConfig.Path, tmpDir)
		cloneOptions := &git.CloneOptions{
			URL:   projectsConfig.Path,
			Depth: 1,
		}

		// authenticate with CI job token if running in a GitLab CI pipeline
		ciJobToken := os.Getenv("CI_JOB_TOKEN")
		if ciJobToken != "" {
			cloneOptions.Auth = &http.BasicAuth{
				Username: "gitlab-ci-token", // this can be anything except an empty string
				Password: ciJobToken,
			}
		}

		_, err = git.PlainClone(tmpDir, false, cloneOptions)
		if err != nil {
			return err
		}
		log.Infof("Successfully cloned %s", projectsConfig.Path)
		projectsConfig.Path = tmpDir
	}

	// detect the project language if not manually set
	if projectsConfig.Language == "" {
		projectLanguage, err := detectLanguage(globalConfig, projectsConfig.Path)
		if err != nil {
			return err
		}
		projectsConfig.Language = projectLanguage
	}

	adapter := getAdapterByName(projectsConfig.Language)
	if adapter == nil {
		return fmt.Errorf("invalid adapter: %s", projectsConfig.Language)
	}

	projectPath := projectsConfig.Path
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
	branchExists, err := checkIfBranchExists(repo, branchName)
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
		log.Errorf("No version found in CHANGELOG.md for project at %s\n", projectsConfig.Path)
		return err
	}

	projectsConfig.NewVersion = version.String()
	log.Infof("Updating version to %s", projectsConfig.NewVersion)
	err = updateVersion(adapter, projectPath, projectsConfig)
	if err != nil {
		return err
	}

	versionFile, err := adapter.VersionFile(projectsConfig)
	if err != nil {
		return err
	}

	// get version file relative path
	versionFileRelativePath, err := filepath.Rel(projectPath, versionFile)

	log.Infof("Adding version file %s", versionFileRelativePath)
	_, err = w.Add(versionFileRelativePath)
	if err != nil {
		return err
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

	globalGitConfig, err := getGlobalGitConfig()
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

	commitMessage := "chore(bump) bump to version " + projectsConfig.NewVersion
	commit, err := commitChanges(
		w,
		commitMessage,
		signKey,
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
		err = pushChangesHttps(repo, cfg, refSpec, globalConfig)
	}

	if err != nil {
		return err
	}

	serviceType, err := getRemoteServiceType(repo)
	if err != nil {
		return err
	}

	if serviceType == "GitLab" {
		err = createGitLabMergeRequest(globalConfig, repo, branchName, projectsConfig.NewVersion)
		if err != nil {
			return err
		}
	}

	return nil
}

func iterateProjects(globalConfig *GlobalConfig) {
	for _, project := range globalConfig.ProjectsConfig {

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
