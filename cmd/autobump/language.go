package main

type Language interface {
	GetProjectName() (string, error)
}

func getLanguageInterface(projectConfig ProjectConfig, languageInterface *Language) {
	if projectConfig.Language == "python" {
		*languageInterface = &Python{ProjectConfig: projectConfig}
	}
}
