package controller

import (
	"crawler/baseline/internal/entity"
	"crawler/baseline/internal/model"
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
}

func NewReleaseController(log *logrus.Logger, db *gorm.DB,
	releaseUsecase *usecase.ReleaseUsecase, releaseScrape *scrape.ReleaseScrape) *ReleaseController {
	return &ReleaseController{
		log:            log,
		db:             db,
		releaseUsecase: releaseUsecase,
		releaseScrape:  releaseScrape,
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

func (c *ReleaseController) CrawlAllReleases(w http.ResponseWriter, r *http.Request) {
	// Create operation timer
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

	// Create response slice
	releaseResponses := make([]*model.ReleaseResponse, 0)
	totalScrapeTime := time.Duration(0)
	totalDbTime := time.Duration(0)

	// Process each repository
	for i, repo := range repoEntities {
		repoStartTime := time.Now()
		repoOwner := repo.UserName
		repoName := repo.RepoName
		repoID := repo.ID

		// Log repository processing start
		c.log.WithFields(logrus.Fields{
			"progress": fmt.Sprintf("%d/%d", i+1, repoCount),
			"owner":    repoOwner,
			"name":     repoName,
			"id":       repoID,
			"phase":    "repo_processing_start",
		}).Info("Processing repository")

		// Scrape releases (measure scraping time)
		scrapeStartTime := time.Now()
		releases := c.releaseScrape.CrawlReleases(repoOwner, repoName)
		scrapeTime := time.Since(scrapeStartTime)
		totalScrapeTime += scrapeTime

		// Log scraping results
		releaseFoundCount := len(releases)
		releaseCount += releaseFoundCount
		c.log.WithFields(logrus.Fields{
			"owner":          repoOwner,
			"name":           repoName,
			"releases_found": releaseFoundCount,
			"scrape_time_ms": scrapeTime.Milliseconds(),
			"phase":          "repo_scraping_complete",
		}).Info("Repository releases scraped")

		// Skip if no releases were found
		if len(releases) == 0 {
			continue
		}

		// Save releases to database using batch insert
		dbStartTime := time.Now()

		// Prepare the release requests as a batch
		releaseRequests := make([]*model.CreateReleaseRequest, 0, len(releases))
		for tag, content := range releases {
			releaseRequests = append(releaseRequests, &model.CreateReleaseRequest{
				TagName: tag,
				Content: content,
				RepoID:  repoID,
			})
		}

		// Batch create all releases for this repository
		batchResponses, err := c.releaseUsecase.BatchCreate(r.Context(), releaseRequests)
		if err != nil {
			c.log.WithFields(logrus.Fields{
				"repo":  repoName,
				"error": err.Error(),
			}).Error("Failed to batch save releases")
			errorCount += len(releaseRequests)
			continue
		}

		// Add successful responses to the main response list
		releaseResponses = append(releaseResponses, batchResponses...)
		successCount += len(batchResponses)

		// Calculate database time
		dbTime := time.Since(dbStartTime)
		totalDbTime += dbTime
		repoTotalTime := time.Since(repoStartTime)

		// Log repository complete
		c.log.WithFields(logrus.Fields{
			"owner":          repoOwner,
			"name":           repoName,
			"scrape_time_ms": scrapeTime.Milliseconds(),
			"db_time_ms":     dbTime.Milliseconds(),
			"total_time_ms":  repoTotalTime.Milliseconds(),
			"success_count":  len(batchResponses),
			"phase":          "repo_processing_complete",
		}).Info("Repository processing completed")
	}

	// Calculate total times
	totalTime := time.Since(startTime)

	// Log completion
	c.log.WithFields(logrus.Fields{
		"total_time_ms":        totalTime.Milliseconds(),
		"total_scrape_time_ms": totalScrapeTime.Milliseconds(),
		"total_db_time_ms":     totalDbTime.Milliseconds(),
		"repos_processed":      repoCount,
		"releases_total":       releaseCount,
		"success_count":        successCount,
		"error_count":          errorCount,
		"phase":                "operation_complete",
	}).Info("Release crawling operation completed")

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(model.WebResponse[[]*model.ReleaseResponse]{
		Data: releaseResponses,
	}); err != nil {
		c.log.WithError(err).Error("Error encoding response")
		http.Error(w, "Error processing response", http.StatusInternalServerError)
	}
}
