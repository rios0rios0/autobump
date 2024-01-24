package main

type Language interface {
	GetProjectName() (string, error)
}

func getLanguageInterface(projectConfig ProjectConfig) Language {
	switch projectConfig.Language {
	case "python":
		return Python{ProjectConfig: projectConfig}
	default:
		return nil
	}
}
