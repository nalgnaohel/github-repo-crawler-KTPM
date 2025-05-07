package config

import (
	"crawler/baseline/internal/http/controller"
	"crawler/baseline/internal/http/route"
	"crawler/baseline/internal/queue"
	"crawler/baseline/internal/repository"
	"crawler/baseline/internal/scrape"
	"crawler/baseline/internal/usecase"

	"github.com/go-chi/chi/v5"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type BootstrapConfig struct {
	DB     *gorm.DB
	Log    *logrus.Logger
	Config *viper.Viper
	Colly  *colly.Collector
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

	queueConfig := queue.NewQueueConfig(config.Config, logConfig.MainLogger)

	// Initialize queue processors
	repoQueueProcessor := queue.NewRepoQueueProcessor(
		logConfig.RepoLogger,
		config.DB,
		repoUsecase,
		queueConfig.MaxSize,
		queueConfig.Workers.Repo,
		queueConfig.BatchSize.Max,
	)
	repoQueueProcessor.Start()

	releaseQueueProcessor := queue.NewReleaseQueueProcessor(
		logConfig.ReleaseLogger,
		config.DB,
		releaseUsecase,
		queueConfig.MaxSize,
		queueConfig.Workers.Release,
		queueConfig.BatchSize.Max,
	)
	releaseQueueProcessor.Start()

	commitQueueProcessor := queue.NewCommitQueueProcessor(
		logConfig.CommitLogger,
		config.DB,
		commitUsecase,
		queueConfig.MaxSize,
		queueConfig.Workers.Commit,
		queueConfig.BatchSize.Max,
	)
	commitQueueProcessor.Start()

	// Initialize scrape services
	repoScrape := scrape.NewRepoScrape(logConfig.RepoLogger, config.Colly)
	releaseScrape := scrape.NewReleaseScrape(logConfig.ReleaseLogger, config.Colly)
	commitScrape := scrape.NewCommitScrape(logConfig.CommitLogger, config.Colly)

	// Initialize controllers
	repoController := controller.NewRepoController(
		logConfig.RepoLogger,
		config.DB,
		repoUsecase,
		repoScrape,
		repoQueueProcessor,
	)

	releaseController := controller.NewReleaseController(
		logConfig.ReleaseLogger,
		config.DB,
		releaseUsecase,
		releaseScrape,
		releaseQueueProcessor,
	)

	commitController := controller.NewCommitController(
		logConfig.CommitLogger,
		config.DB,
		commitUsecase,
		commitScrape,
		commitQueueProcessor,
	)

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
