package main

import (
	"github.com/rios0rios0/autobump/internal"
	"github.com/rios0rios0/autobump/internal/infrastructure/controllers"
	infraRepos "github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"go.uber.org/dig"
)

func injectAppContext() *internal.AppInternal {
	container := dig.New()

	if err := internal.RegisterProviders(container); err != nil {
		panic(err)
	}

	var appInternal *internal.AppInternal
	if err := container.Invoke(func(ai *internal.AppInternal) {
		appInternal = ai
	}); err != nil {
		panic(err)
	}

	return appInternal
}

func injectSingleController() *controllers.SingleController {
	container := dig.New()

	if err := internal.RegisterProviders(container); err != nil {
		panic(err)
	}

	var controller *controllers.SingleController
	if err := container.Invoke(func(c *controllers.SingleController) {
		controller = c
	}); err != nil {
		panic(err)
	}

	return controller
}

func injectProviderRegistry() *infraRepos.GitServiceRegistry {
	container := dig.New()

	if err := internal.RegisterProviders(container); err != nil {
		panic(err)
	}

	var registry *infraRepos.GitServiceRegistry
	if err := container.Invoke(func(r *infraRepos.GitServiceRegistry) {
		registry = r
	}); err != nil {
		panic(err)
	}

	return registry
}
