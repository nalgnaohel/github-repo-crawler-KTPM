package controller

import (
	"crawler/baseline/internal/entity"
	"crawler/baseline/internal/model"
	"crawler/baseline/internal/queue"
	"crawler/baseline/internal/repository"
	"crawler/baseline/internal/scrape"
	"crawler/baseline/internal/usecase"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ReleaseController struct {
	log            *logrus.Logger
	db             *gorm.DB
	releaseUsecase *usecase.ReleaseUsecase
	releaseScrape  *scrape.ReleaseScrape
	queueProcessor *queue.ReleaseQueueProcessor
}

func NewReleaseController(log *logrus.Logger, db *gorm.DB,
	releaseUsecase *usecase.ReleaseUsecase,
	releaseScrape *scrape.ReleaseScrape,
	queueProcessor *queue.ReleaseQueueProcessor) *ReleaseController {

	return &ReleaseController{
		log:            log,
		db:             db,
		releaseUsecase: releaseUsecase,
		releaseScrape:  releaseScrape,
		queueProcessor: queueProcessor,
	}
}

// Modify CrawlAllReleases to use the queue processor
func (c *ReleaseController) CrawlAllReleases(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	c.log.WithField("phase", "start").Info("Starting release crawling operation")

	// Metrics tracking
	successCount := 0
	errorCount := 0
	repoCount := 0
	releaseCount := 0

	// Get all repositories
	repoFetchStartTime := time.Now()
	c.log.WithField("phase", "fetching_repositories").Info("Fetching repositories from database")

	repoEntities := []entity.Repository{}
	repoRepository := repository.NewRepoRepository(c.log)
	err := repoRepository.FindAll(c.db, &repoEntities)
	if err != nil {
		c.log.WithError(err).Error("Error fetching repositories")
		http.Error(w, "Error fetching repositories", http.StatusInternalServerError)
		return
	}

	// Track repository fetch time
	repoFetchTime := time.Since(repoFetchStartTime)
	repoCount = len(repoEntities)
	c.log.WithFields(logrus.Fields{
		"repo_count":  repoCount,
		"duration_ms": repoFetchTime.Milliseconds(),
		"phase":       "repositories_loaded",
	}).Info("Repositories loaded from database")

	totalScrapeTime := time.Duration(0)
	totalQueueTime := time.Duration(0)

	// Check if queue processor is available
	if c.queueProcessor == nil {
		c.log.Warn("Queue processor is not available, using synchronous processing")
	}

	// Process each repository
	for i, repo := range repoEntities {
		repoStartTime := time.Now()
		repoOwner := repo.UserName
		repoName := repo.RepoName
		repoID := repo.ID

		c.log.WithFields(logrus.Fields{
			"progress": fmt.Sprintf("%d/%d", i+1, repoCount),
			"owner":    repoOwner,
			"name":     repoName,
			"id":       repoID,
			"phase":    "repo_processing_start",
		}).Info("Processing repository")

		// Scrape releases
		scrapeStartTime := time.Now()

		// Use releaseScrape if available, fall back to static function
		var releases map[string]string
		releases = c.releaseScrape.CrawlReleases(repoOwner, repoName)

		scrapeTime := time.Since(scrapeStartTime)
		totalScrapeTime += scrapeTime

		releaseFoundCount := len(releases)
		releaseCount += releaseFoundCount
		c.log.WithFields(logrus.Fields{
			"owner":          repoOwner,
			"name":           repoName,
			"releases_found": releaseFoundCount,
			"scrape_time_ms": scrapeTime.Milliseconds(),
			"phase":          "repo_scraping_complete",
		}).Info("Repository releases scraped")

		// Prepare release requests
		queueStartTime := time.Now()
		releaseRequests := make([]*model.CreateReleaseRequest, 0, releaseFoundCount)

		for tag, content := range releases {
			releaseRequests = append(releaseRequests, &model.CreateReleaseRequest{
				TagName: tag,
				Content: content,
				RepoID:  repoID,
			})
		}

		// Process using queue if available, otherwise use direct method
		repoSuccessCount := 0
		repoErrorCount := 0

		if c.queueProcessor != nil {
			// Queue the releases for asynchronous processing
			enqueued := c.queueProcessor.BatchEnqueueReleases(releaseRequests)
			repoSuccessCount = enqueued
			repoErrorCount = releaseFoundCount - enqueued

			successCount += enqueued
			errorCount += repoErrorCount
		} else {
			// Process synchronously
			for _, request := range releaseRequests {
				_, err := c.releaseUsecase.Create(r.Context(), request)
				if err != nil {
					c.log.WithFields(logrus.Fields{
						"repo":  repoName,
						"tag":   request.TagName,
						"error": err.Error(),
					}).Error("Failed to save release")
					repoErrorCount++
					errorCount++
					continue
				}

				repoSuccessCount++
				successCount++
			}
		}

		queueTime := time.Since(queueStartTime)
		totalQueueTime += queueTime
		repoTotalTime := time.Since(repoStartTime)

		c.log.WithFields(logrus.Fields{
			"owner":          repoOwner,
			"name":           repoName,
			"releases_found": releaseFoundCount,
			"success_count":  repoSuccessCount,
			"error_count":    repoErrorCount,
			"scrape_time_ms": scrapeTime.Milliseconds(),
			"queue_time_ms":  queueTime.Milliseconds(),
			"total_time_ms":  repoTotalTime.Milliseconds(),
			"phase":          "repo_processing_complete",
		}).Info("Repository processing completed")
	}

	// Calculate total times
	totalTime := time.Since(startTime)

	// Prepare queue metrics for response
	queueSize := 0
	processingCount := 0
	if c.queueProcessor != nil {
		queueSize = c.queueProcessor.GetQueueSize()
		processingCount = c.queueProcessor.GetProcessingCount()
	}

	// Log completion
	c.log.WithFields(logrus.Fields{
		"total_time_ms":        totalTime.Milliseconds(),
		"total_scrape_time_ms": totalScrapeTime.Milliseconds(),
		"total_queue_time_ms":  totalQueueTime.Milliseconds(),
		"repos_processed":      repoCount,
		"releases_total":       releaseCount,
		"success_count":        successCount,
		"error_count":          errorCount,
		"queue_size":           queueSize,
		"processing_count":     processingCount,
		"phase":                "operation_complete",
	}).Info("Release crawling operation completed")

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(model.WebResponse[map[string]interface{}]{
		Data: map[string]interface{}{
			"repos_processed":  repoCount,
			"releases_found":   releaseCount,
			"releases_queued":  successCount,
			"error_count":      errorCount,
			"queue_size":       queueSize,
			"processing_count": processingCount,
		},
	}); err != nil {
		c.log.WithError(err).Error("Error encoding response")
		http.Error(w, "Error processing response", http.StatusInternalServerError)
	}
}

func (c *ReleaseController) GetRelease(w http.ResponseWriter, r *http.Request) {
	// Extract releaseID from URL parameters
	releaseID, err := strconv.Atoi(chi.URLParam(r, "releaseID"))
	if err != nil {
		c.log.WithError(err).Error("Invalid release ID format")
		http.Error(w, "Invalid release ID", http.StatusBadRequest)
		return
	}

	c.log.WithField("release_id", releaseID).Info("Fetching release")

	// Create release repository instance
	releaseRepository := repository.NewReleaseRepository(c.log)

	// Find release by ID
	releaseEntity := &entity.Release{}
	err = releaseRepository.FindById(c.db, releaseEntity, releaseID)

	if err != nil {
		c.log.WithError(err).WithField("release_id", releaseID).Error("Release not found")
		http.Error(w, "Release not found", http.StatusNotFound)
		return
	}

	// Convert entity to response model
	releaseResponse := &model.ReleaseResponse{
		ID:      releaseEntity.ID,
		TagName: releaseEntity.TagName,
		Content: releaseEntity.Content,
		RepoID:  releaseEntity.RepoID,
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(releaseResponse); err != nil {
		c.log.WithError(err).Error("Error encoding response")
		http.Error(w, "Error processing response", http.StatusInternalServerError)
		return
	}
}
