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

func injectLocalController() *controllers.LocalController {
	container := dig.New()

	if err := internal.RegisterProviders(container); err != nil {
		panic(err)
	}

	var controller *controllers.LocalController
	if err := container.Invoke(func(c *controllers.LocalController) {
		controller = c
	}); err != nil {
		panic(err)
	}

	return controller
}

func injectProviderRegistry() *infraRepos.ProviderRegistry {
	container := dig.New()

	if err := internal.RegisterProviders(container); err != nil {
		panic(err)
	}

	var registry *infraRepos.ProviderRegistry
	if err := container.Invoke(func(r *infraRepos.ProviderRegistry) {
		registry = r
	}); err != nil {
		panic(err)
	}

	return registry
}
