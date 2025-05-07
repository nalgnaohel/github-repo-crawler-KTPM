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
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type CommitController struct {
	log           *logrus.Logger
	db            *gorm.DB
	commitUsecase *usecase.CommitUsecase
}

func NewCommitController(log *logrus.Logger, db *gorm.DB,
	commitUsecase *usecase.CommitUsecase) *CommitController {
	return &CommitController{
		log:           log,
		db:            db,
		commitUsecase: commitUsecase,
	}
}

func (c *CommitController) GetCommit(w http.ResponseWriter, r *http.Request) {
	commitID, _ := strconv.Atoi(chi.URLParam(r, "commitID"))

	c.log.Infof("Fetching commit with ID: %d", commitID)

	commitRepository := repository.NewCommitRepository(c.log)

	commitEntity := &entity.Commit{}
	err := commitRepository.FindById(c.db, commitEntity, commitID)

	if err != nil {
		c.log.WithError(err).Errorf("Error finding commit with ID %d", commitID)
		http.Error(w, "Commit not found", http.StatusNotFound)
		return
	}

	commitResponse := &model.CommitResponse{
		ID:        commitEntity.ID,
		Hash:      commitEntity.Hash,
		Message:   commitEntity.Message,
		ReleaseID: commitEntity.ReleaseID,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(commitResponse); err != nil {
		c.log.WithError(err).Error("Error encoding commit response")
		http.Error(w, "Error processing response", http.StatusInternalServerError)
		return
	}
}

func (c *CommitController) GetCommitsByRelease(w http.ResponseWriter, r *http.Request) {
	releaseID, _ := strconv.Atoi(chi.URLParam(r, "releaseID"))

	c.log.Infof("Fetching commits for release ID: %d", releaseID)

	// Get commits for this release
	commits, err := c.commitUsecase.GetCommitsByReleaseID(r.Context(), int64(releaseID))
	if err != nil {
		c.log.WithError(err).Errorf("Error fetching commits for release ID %d", releaseID)
		http.Error(w, "Failed to retrieve commits", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(model.WebResponse[[]*model.CommitResponse]{
		Data: commits,
	}); err != nil {
		c.log.WithError(err).Error("Error encoding commits response")
		http.Error(w, "Error processing response", http.StatusInternalServerError)
		return
	}
}

func (c *CommitController) CrawlCommitsByRelease(w http.ResponseWriter, r *http.Request) {
	releaseID, _ := strconv.Atoi(chi.URLParam(r, "releaseID"))

	c.log.WithFields(logrus.Fields{
		"release_id": releaseID,
		"phase":      "start",
	}).Info("Starting commit crawling for release")

	// Get the release information first
	releaseRepository := repository.NewReleaseRepository(c.log)
	releaseEntity := &entity.Release{}
	if err := releaseRepository.FindById(c.db, releaseEntity, releaseID); err != nil {
		c.log.WithError(err).Errorf("Error finding release with ID %d", releaseID)
		http.Error(w, "Release not found", http.StatusNotFound)
		return
	}

	// Get the repo information associated with this release
	repoRepository := repository.NewRepoRepository(c.log)
	repoEntity := &entity.Repository{}
	if err := repoRepository.FindById(c.db, repoEntity, releaseEntity.RepoID); err != nil {
		c.log.WithError(err).Errorf("Error finding repository with ID %d", releaseEntity.RepoID)
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	startTime := time.Now()

	// Get all commits for this release
	c.log.WithFields(logrus.Fields{
		"release_tag": releaseEntity.TagName,
		"repo":        fmt.Sprintf("%s/%s", repoEntity.UserName, repoEntity.RepoName),
		"phase":       "scraping",
	}).Info("Crawling commits")

	// Crawl commits
	commitStrings := scrape.CrawlCommit(repoEntity.UserName, repoEntity.RepoName, releaseEntity.TagName)
	scrapeTime := time.Since(startTime)

	c.log.WithFields(logrus.Fields{
		"commit_count": len(commitStrings),
		"duration_ms":  scrapeTime.Milliseconds(),
		"phase":        "scraping_complete",
	}).Info("Commit crawling completed")

	// Process and save the commits
	dbStartTime := time.Now()
	commitRequests := make([]*model.CreateCommitRequest, 0, len(commitStrings))

	// Parse the commit strings and create requests
	for _, commitStr := range commitStrings {
		// Extract hash and message from the string (format: "Hash: <hash> - Message: <message>")
		parts := strings.SplitN(commitStr, " - Message: ", 2)
		if len(parts) != 2 {
			c.log.WithField("commit_str", commitStr).Warn("Invalid commit string format")
			continue
		}

		hash := strings.TrimPrefix(parts[0], "Hash: ")
		message := parts[1]

		commitRequests = append(commitRequests, &model.CreateCommitRequest{
			Hash:      hash,
			Message:   message,
			ReleaseID: releaseEntity.ID,
		})
	}

	// Batch create the commits
	responses, err := c.commitUsecase.BatchCreate(r.Context(), commitRequests)
	if err != nil {
		c.log.WithError(err).Error("Error saving commits")
		http.Error(w, "Failed to save commits", http.StatusInternalServerError)
		return
	}

	dbTime := time.Since(dbStartTime)
	totalTime := time.Since(startTime)

	c.log.WithFields(logrus.Fields{
		"scrape_time_ms": scrapeTime.Milliseconds(),
		"db_time_ms":     dbTime.Milliseconds(),
		"total_time_ms":  totalTime.Milliseconds(),
		"commit_count":   len(responses),
		"phase":          "complete",
	}).Info("Commit crawling and saving completed")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(model.WebResponse[[]*model.CommitResponse]{
		Data: responses,
	}); err != nil {
		c.log.WithError(err).Error("Error encoding response")
		http.Error(w, "Error processing response", http.StatusInternalServerError)
	}
}

func (c *CommitController) CrawlAllCommits(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	c.log.WithField("phase", "start").Info("Starting crawling commits for all releases")

	// Metrics tracking
	successCount := 0
	errorCount := 0
	releaseCount := 0
	commitCount := 0

	// Get all releases
	var releases []entity.Release
	if err := c.db.Find(&releases).Error; err != nil {
		c.log.WithError(err).Error("Error fetching all releases")
		http.Error(w, "Error fetching releases", http.StatusInternalServerError)
		return
	}

	releaseCount = len(releases)
	c.log.WithFields(logrus.Fields{
		"release_count": releaseCount,
		"duration_ms":   time.Since(startTime).Milliseconds(),
		"phase":         "releases_loaded",
	}).Info("Releases loaded from database")

	// Process each release
	for i, release := range releases {
		releaseStartTime := time.Now()

		// Get the repository for this release
		repoEntity := &entity.Repository{}
		if err := c.db.First(repoEntity, release.RepoID).Error; err != nil {
			c.log.WithFields(logrus.Fields{
				"release_id": release.ID,
				"repo_id":    release.RepoID,
				"error":      err.Error(),
			}).Error("Failed to find repository for release")
			errorCount++
			continue
		}

		// Log processing start
		c.log.WithFields(logrus.Fields{
			"progress":   fmt.Sprintf("%d/%d", i+1, releaseCount),
			"release_id": release.ID,
			"tag":        release.TagName,
			"repo":       fmt.Sprintf("%s/%s", repoEntity.UserName, repoEntity.RepoName),
		}).Info("Processing release")

		// Crawl commits for this release
		scrapeStartTime := time.Now()
		commitStrings := scrape.CrawlCommit(repoEntity.UserName, repoEntity.RepoName, release.TagName)
		scrapeTime := time.Since(scrapeStartTime)

		releaseCommitCount := len(commitStrings)
		commitCount += releaseCommitCount

		c.log.WithFields(logrus.Fields{
			"release_id":     release.ID,
			"tag":            release.TagName,
			"commits_found":  releaseCommitCount,
			"scrape_time_ms": scrapeTime.Milliseconds(),
		}).Info("Commits scraped")

		// Process and save commits
		dbStartTime := time.Now()
		releaseSuccessCount := 0
		releaseErrorCount := 0

		commitRequests := make([]*model.CreateCommitRequest, 0, releaseCommitCount)

		// Parse commit strings
		for _, commitStr := range commitStrings {
			parts := strings.SplitN(commitStr, " - Message: ", 2)
			if len(parts) != 2 {
				c.log.WithField("commit_str", commitStr).Warn("Invalid commit string format")
				continue
			}

			hash := strings.TrimPrefix(parts[0], "Hash: ")
			message := parts[1]

			commitRequests = append(commitRequests, &model.CreateCommitRequest{
				Hash:      hash,
				Message:   message,
				ReleaseID: release.ID,
			})
		}

		// Batch create if we have commits
		if len(commitRequests) > 0 {
			_, err := c.commitUsecase.BatchCreate(r.Context(), commitRequests)
			if err != nil {
				c.log.WithFields(logrus.Fields{
					"release_id": release.ID,
					"tag":        release.TagName,
					"error":      err.Error(),
				}).Error("Failed to save commits")
				releaseErrorCount += len(commitRequests)
				errorCount += len(commitRequests)
			} else {
				releaseSuccessCount = len(commitRequests)
				successCount += len(commitRequests)
			}
		}

		dbTime := time.Since(dbStartTime)
		releaseTotalTime := time.Since(releaseStartTime)

		c.log.WithFields(logrus.Fields{
			"release_id":     release.ID,
			"tag":            release.TagName,
			"scrape_time_ms": scrapeTime.Milliseconds(),
			"db_time_ms":     dbTime.Milliseconds(),
			"total_time_ms":  releaseTotalTime.Milliseconds(),
			"success_count":  releaseSuccessCount,
			"error_count":    releaseErrorCount,
		}).Info("Release processing completed")
	}

	// Log completion
	totalTime := time.Since(startTime)
	c.log.WithFields(logrus.Fields{
		"total_time_ms":      totalTime.Milliseconds(),
		"releases_processed": releaseCount,
		"commits_total":      commitCount,
		"success_count":      successCount,
		"error_count":        errorCount,
	}).Info("Commit crawling operation completed")

	// Send response
	w.Header().Set("Content-Type", "application/json")
	response := model.WebResponse[map[string]interface{}]{
		Data: map[string]interface{}{
			"releases_processed": releaseCount,
			"commits_found":      commitCount,
			"commits_saved":      successCount,
			"errors":             errorCount,
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		c.log.WithError(err).Error("Error encoding response")
		http.Error(w, "Error processing response", http.StatusInternalServerError)
	}
}
