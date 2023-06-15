package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5/config"
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
		fmt.Printf("No version found in CHANGELOG.md for project at %s\n", projectsConfig.Path)
		return err
	}

	projectsConfig.NewVersion = version.String()
	log.Infof("Updating version to %s", projectsConfig.NewVersion)
	err = updateVersion(adapter, projectPath, projectsConfig)
	if err != nil {
		return err
	}
	log.Infof("Adding version file %s", adapter.VersionFile(projectsConfig))
	_, err = w.Add(adapter.VersionFile(projectsConfig))
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
		err = createGitLabMergeRequest(globalConfig, repo, branchName)
		if err != nil {
			return err
		}
	}

	return nil
}

func iterateProjects(globalConfig *GlobalConfig) error {
	for _, project := range globalConfig.ProjectsConfig {
		err := processRepo(globalConfig, &project)
		if err != nil {
			log.Errorf("Error processing project at %s: %v\n", project.Path, err)
		}
	}
	return nil
}
