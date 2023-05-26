package main

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	log "github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

type GlobalConfig struct {
	ProjectsConfig []ProjectsConfig
	GitLabConfig   GitLabConfig
}

type ProjectsConfig struct {
	Path       string `yaml:"path"`
	Language   string `yaml:"language"`
	NewVersion string
}

type GitLabConfig struct {
	UserName          string `yaml:"username"`
	Email             string `yaml:"email"`
	GitLabAccessToken string `yaml:"gitlab_access_token"`
}

// LanguageAdapter is the interface for language-specific adapters
type LanguageAdapter interface {
	UpdateVersion(path string, config *ProjectsConfig) error
	VersionFile() string
	VersionIdentifier() string
}

// PythonAdapter is the adapter for Python projects
type PythonAdapter struct{}

func (p *PythonAdapter) UpdateVersion(path string, config *ProjectsConfig) error {
	projectName := filepath.Base(config.Path)
	versionFilePath := filepath.Join(path, projectName, p.VersionFile())
	if _, err := os.Stat(versionFilePath); os.IsNotExist(err) {
		return nil
	}

	content, err := ioutil.ReadFile(versionFilePath)
	if err != nil {
		return err
	}

	versionIdentifier := p.VersionIdentifier()
	versionPattern := fmt.Sprintf(`%s(\d+\.\d+\.\d+)`, regexp.QuoteMeta(versionIdentifier))
	re := regexp.MustCompile(versionPattern)

	updatedContent := re.ReplaceAllString(string(content), versionIdentifier+config.NewVersion)
	err = ioutil.WriteFile(versionFilePath, []byte(updatedContent), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (p *PythonAdapter) VersionFile() string {
	return "__init__.py"
}

func (p *PythonAdapter) VersionIdentifier() string {
	return "__version__ = "
}

func getRemoteServiceType(repo *git.Repository) (string, error) {
	cfg, err := repo.Config()
	if err != nil {
		return "", err
	}

	for _, remote := range cfg.Remotes {
		if strings.Contains(remote.URLs[0], "gitlab.com") {
			return "GitLab", nil
		} else if strings.Contains(remote.URLs[0], "github.com") {
			return "GitHub", nil
		}
	}

	return "Unknown", nil
}

func createGitLabMergeRequest(globalConfig *GlobalConfig, projectPath string, repo *git.Repository) error {
	gitlabClient, err := gitlab.NewClient(globalConfig.GitLabConfig.GitLabAccessToken)
	if err != nil {
		return err
	}

	remoteURL, err := getRemoteServiceType(repo)
	if err != nil {
		return err
	}

	remoteURLParsed, err := url.Parse(remoteURL)
	if err != nil {
		return err
	}

	namespace, project := filepath.Split(remoteURLParsed.Path)
	namespace = strings.TrimSuffix(namespace, "/")

	projectID := url.PathEscape(fmt.Sprintf("%s/%s", namespace, project))
	mrTitle := "Bump version"

	mergeRequestOptions := &gitlab.CreateMergeRequestOptions{
		SourceBranch:       gitlab.String("chore/bump"),
		TargetBranch:       gitlab.String("main"),
		Title:              &mrTitle,
		RemoveSourceBranch: gitlab.Bool(true),
	}

	_, _, err = gitlabClient.MergeRequests.CreateMergeRequest(projectID, mergeRequestOptions)
	if err != nil {
		return err
	}

	fmt.Printf("Merge Request created for project at %s\n", projectPath)
	return nil
}

func processProject(globalConfig *GlobalConfig, config *ProjectsConfig) error {
	log.Info("Getting adapter by name")
	adapter := getAdapterByName(config.Language)
	if adapter == nil {
		return fmt.Errorf("invalid adapter: %s", config.Language)
	}

	log.Info("Joining project path")
	projectPath := config.Path

	log.Info("Joining changelog path")
	changelogPath := filepath.Join(projectPath, "CHANGELOG.md")
	version, err := UpdateChangelogFile(changelogPath)
	if err != nil {
		fmt.Printf("No version found in CHANGELOG.md for project at %s\n", config.Path)
		return err
	}

	log.Info("Updating adapter version")
	config.NewVersion = version.String()
	err = adapter.UpdateVersion(projectPath, config)
	if err != nil {
		return err
	}

	log.Info("Opening git repository")
	repo, err := git.PlainOpen(projectPath)
	if err != nil {
		return err
	}

	log.Info("Getting worktree")
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	log.Info("Getting CHANGELOG.md's path relative to repository root")
	changelogRelativePath, err := filepath.Rel(changelogPath, projectPath)
	if err != nil {
		return err
	}

	log.Info("Adding version file to the worktree")
	result, err := w.Add(changelogRelativePath)
	if err != nil {
		log.Errorf("Result not expected: %v", result)
		return err
	}

	log.Info("Committing the updated version")
	commit, err := w.Commit("Bump version to "+config.NewVersion, &git.CommitOptions{
		Author: &object.Signature{
			Name:  globalConfig.GitLabConfig.UserName,
			Email: globalConfig.GitLabConfig.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	log.Info("Committing the object")
	_, err = repo.CommitObject(commit)
	if err != nil {
		return err
	}

	log.Info("Getting repository head")
	head, err := repo.Head()
	if err != nil {
		return err
	}

	log.Info("Creating hash reference")
	ref := plumbing.NewHashReference("refs/heads/chore/bump", head.Hash())
	err = repo.Storer.SetReference(ref)
	if err != nil {
		return err
	}

	log.Info("Determining remote service type")
	serviceType, err := getRemoteServiceType(repo)
	if err != nil {
		return err
	}

	if serviceType == "GitLab" {
		log.Info("Creating GitLab merge request")
		err = createGitLabMergeRequest(globalConfig, projectPath, repo)
		if err != nil {
			return err
		}
	}

	log.Info("Project processing completed")
	return nil
}

func getAdapterByName(name string) LanguageAdapter {
	switch name {
	case "Python":
		return &PythonAdapter{}
	default:
		return nil
	}
}

func iterateProjects(globalConfig *GlobalConfig) error {
	for _, project := range globalConfig.ProjectsConfig {
		err := processProject(globalConfig, &project)
		if err != nil {
			fmt.Printf("Error processing project at %s: %v\n", project.Path, err)
		}
	}
	return nil
}

func main() {
	data, err := ioutil.ReadFile("configs/autobump.yaml")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var globalConfig GlobalConfig
	err = yaml.Unmarshal(data, &globalConfig)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(2)
	}

	err = iterateProjects(&globalConfig)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
