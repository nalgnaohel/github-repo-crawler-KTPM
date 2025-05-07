package config

import (
	"crawler/baseline/internal/http/controller"
	"crawler/baseline/internal/http/route"
	"crawler/baseline/internal/repository"
	"crawler/baseline/internal/usecase"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type BootstrapConfig struct {
	DB     *gorm.DB
	Log    *logrus.Logger
	Config *viper.Viper
}

func Bootstrap(config *BootstrapConfig) *chi.Mux {
	// Set up loggers
	logConfig := SetupLoggers()

	// Store main logger in config
	config.Log = logConfig.MainLogger

	// Initialize repositories
	repoRepository := repository.NewRepoRepository(logConfig.RepoLogger)
	releaseRepository := repository.NewReleaseRepository(logConfig.ReleaseLogger)
	commitRepository := repository.NewCommitRepository(logConfig.CommitLogger)

	// Initialize usecases
	repoUsecase := usecase.NewRepoUsecase(config.DB, logConfig.RepoLogger, repoRepository)
	releaseUsecase := usecase.NewReleaseUsecase(config.DB, logConfig.ReleaseLogger, releaseRepository)
	commitUsecase := usecase.NewCommitUsecase(config.DB, logConfig.CommitLogger, commitRepository)
	// Initialize controllers
	repoController := controller.NewRepoController(logConfig.RepoLogger, config.DB, repoUsecase)
	releaseController := controller.NewReleaseController(logConfig.ReleaseLogger, config.DB, releaseUsecase)
	commitController := controller.NewCommitController(logConfig.CommitLogger, config.DB, commitUsecase)
	// Setup routes
	route := route.RouteConfig{
		App:               chi.NewRouter(),
		RepoController:    repoController,
		ReleaseController: releaseController,
		CommitController:  commitController,
	}

	r := route.Setup()
	return r
}
