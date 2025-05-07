package controller

import (
	"context"
	"crawler/baseline/internal/entity"
	"crawler/baseline/internal/model"
	"crawler/baseline/internal/queue"
	"crawler/baseline/internal/repository"
	"crawler/baseline/internal/scrape"
	"crawler/baseline/internal/usecase"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type RepoController struct {
	log            *logrus.Logger
	db             *gorm.DB
	repoUsecase    *usecase.RepoUsecase
	repoScrape     *scrape.RepoScrape
	queueProcessor *queue.RepoQueueProcessor
}

func NewRepoController(
	log *logrus.Logger,
	db *gorm.DB,
	repoUsecase *usecase.RepoUsecase,
	repoScrape *scrape.RepoScrape,
	queueProcessor *queue.RepoQueueProcessor) *RepoController {
	return &RepoController{
		log:            log,
		db:             db,
		repoUsecase:    repoUsecase,
		repoScrape:     repoScrape,
		queueProcessor: queueProcessor,
	}
}

func (c *RepoController) RepoCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repoID := chi.URLParam(r, "repoID")
		repoEntity := &entity.Repository{}
		repoRepository := repository.NewRepoRepository(c.log)
		err := repoRepository.FindById(c.db, repoEntity, repoID)
		if err != nil {
			c.log.WithError(err).Errorf("Error finding repo with ID %s", repoID)
			http.Error(w, "Repo not found", http.StatusNotFound)
			return
		}
		repoResponse := model.RepoResponse{
			ID:       repoEntity.ID,
			RepoName: repoEntity.RepoName,
			UserName: repoEntity.UserName,
		}
		ctx := context.WithValue(r.Context(), "repo", repoResponse)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (c *RepoController) GetRepo(w http.ResponseWriter, r *http.Request) {
	// Extract repoID from URL parameters
	repoID, err := strconv.Atoi(chi.URLParam(r, "repoID"))
	if err != nil {
		c.log.WithError(err).Error("Invalid repository ID format")
		http.Error(w, "Invalid repository ID", http.StatusBadRequest)
		return
	}

	c.log.WithField("repo_id", repoID).Info("Fetching repository")

	// Create repository instance
	repoRepository := repository.NewRepoRepository(c.log)

	// Find repository by ID
	repoEntity := &entity.Repository{}
	err = repoRepository.FindById(c.db, repoEntity, repoID)

	if err != nil {
		c.log.WithError(err).WithField("repo_id", repoID).Error("Repository not found")
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// Convert entity to response model
	repoResponse := &model.RepoResponse{
		ID:       repoEntity.ID,
		RepoName: repoEntity.RepoName,
		UserName: repoEntity.UserName,
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(repoResponse); err != nil {
		c.log.WithError(err).Error("Error encoding response")
		http.Error(w, "Error processing response", http.StatusInternalServerError)
		return
	}
}

func (c *RepoController) CrawlAllRepos(w http.ResponseWriter, r *http.Request) {
	// Start timing
	startTime := time.Now()
	c.log.WithField("phase", "start").Info("Starting repository crawling operation")

	// Scraping phase
	scrapeStartTime := time.Now()
	c.log.WithField("phase", "scraping_start").Info("Starting repository scraping")

	repos, err := c.repoScrape.CrawlAllRepos()
	if err != nil {
		c.log.WithError(err).Error("Error crawling repositories")
		http.Error(w, "Failed to crawl repositories", http.StatusInternalServerError)
		return
	}

	scrapeTime := time.Since(scrapeStartTime)
	c.log.WithFields(logrus.Fields{
		"repos_found": len(repos),
		"duration_ms": scrapeTime.Milliseconds(),
		"phase":       "scraping_complete",
	}).Info("Repository scraping completed")

	// Database operations phase
	dbStartTime := time.Now()
	c.log.WithField("phase", "database_start").Info("Starting database operations")

	var successCount int
	var responseData []*model.RepoResponse

	// Check if queue processor is available
	if c.queueProcessor != nil {
		// Use queue for asynchronous processing
		enqueuedCount := c.queueProcessor.BatchEnqueueRepos(repos)
		successCount = enqueuedCount

		c.log.WithFields(logrus.Fields{
			"enqueued": enqueuedCount,
			"total":    len(repos),
		}).Info("Repositories enqueued for processing")

		// For response, we can't return the actual created repos since they're processed async
		// So we'll create a placeholder response
		responseData = make([]*model.RepoResponse, 0)
	} else {
		// Fall back to direct processing
		responseData, err = c.repoUsecase.BatchCreate(r.Context(), repos)
		if err != nil {
			c.log.WithError(err).Error("Failed to create repositories")
			http.Error(w, "Failed to save repositories", http.StatusInternalServerError)
			return
		}
		successCount = len(responseData)
	}

	dbTime := time.Since(dbStartTime)
	totalTime := time.Since(startTime)

	// Get queue metrics if available
	queueSize := 0
	processingCount := 0
	if c.queueProcessor != nil {
		queueSize = c.queueProcessor.GetQueueSize()
		processingCount = c.queueProcessor.GetProcessingCount()
	}

	c.log.WithFields(logrus.Fields{
		"scrape_time_ms":   scrapeTime.Milliseconds(),
		"db_time_ms":       dbTime.Milliseconds(),
		"total_time_ms":    totalTime.Milliseconds(),
		"repos_found":      len(repos),
		"success_count":    successCount,
		"queue_size":       queueSize,
		"processing_count": processingCount,
		"phase":            "operation_complete",
	}).Info("Repository crawling operation completed")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(model.WebResponse[map[string]interface{}]{
		Data: map[string]interface{}{
			"repos_found":      len(repos),
			"repos_enqueued":   successCount,
			"queue_size":       queueSize,
			"processing_count": processingCount,
		},
	}); err != nil {
		c.log.WithError(err).Error("Error encoding response")
		http.Error(w, "Error processing response", http.StatusInternalServerError)
	}
}
