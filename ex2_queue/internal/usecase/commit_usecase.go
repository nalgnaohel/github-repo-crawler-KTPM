package usecase

import (
	"context"
	"crawler/baseline/internal/entity"
	"crawler/baseline/internal/model"
	"crawler/baseline/internal/repository"
	"sync"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type CommitUsecase struct {
	DB               *gorm.DB
	Log              *logrus.Logger
	CommitRepository *repository.CommitRepository
}

func NewCommitUsecase(db *gorm.DB, log *logrus.Logger,
	commitRepo *repository.CommitRepository) *CommitUsecase {
	return &CommitUsecase{
		DB:               db,
		Log:              log,
		CommitRepository: commitRepo,
	}
}

func (c *CommitUsecase) Create(ctx context.Context, request *model.CreateCommitRequest) (*model.CommitResponse, error) {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	commit := &entity.Commit{
		Hash:      request.Hash,
		Message:   request.Message,
		ReleaseID: request.ReleaseID,
	}

	// Check if this commit already exists to avoid duplicates
	var existingCommit entity.Commit
	existingCheck := tx.Where("hash = ? AND release_id = ?", commit.Hash, commit.ReleaseID).First(&existingCommit)
	if existingCheck.Error == nil {
		// Commit already exists, return it
		c.Log.WithFields(logrus.Fields{
			"hash":       commit.Hash[:8] + "...",
			"release_id": commit.ReleaseID,
		}).Debug("Commit already exists, skipping")

		return &model.CommitResponse{
			ID:        existingCommit.ID,
			Hash:      existingCommit.Hash,
			Message:   existingCommit.Message,
			ReleaseID: existingCommit.ReleaseID,
		}, nil
	}

	if err := c.CommitRepository.Create(tx, commit); err != nil {
		c.Log.WithError(err).Error("error creating commit")
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.WithError(err).Error("error committing transaction")
		return nil, err
	}

	return &model.CommitResponse{
		ID:        commit.ID,
		Hash:      commit.Hash,
		Message:   commit.Message,
		ReleaseID: commit.ReleaseID,
	}, nil
}

// GetCommitsByReleaseID retrieves all commits for a specific release
func (c *CommitUsecase) GetCommitsByReleaseID(ctx context.Context, releaseID int64) ([]*model.CommitResponse, error) {
	var commits []entity.Commit

	if err := c.DB.WithContext(ctx).Where("release_id = ?", releaseID).Find(&commits).Error; err != nil {
		c.Log.WithError(err).Errorf("Error fetching commits for release ID %d", releaseID)
		return nil, err
	}

	responses := make([]*model.CommitResponse, len(commits))
	for i, commit := range commits {
		responses[i] = &model.CommitResponse{
			ID:        commit.ID,
			Hash:      commit.Hash,
			Message:   commit.Message,
			ReleaseID: commit.ReleaseID,
		}
	}

	return responses, nil
}

// BatchCreate inserts multiple commits in a single transaction
func (c *CommitUsecase) BatchCreate(ctx context.Context, requests []*model.CreateCommitRequest) ([]*model.CommitResponse, error) {
	if len(requests) == 0 {
		return []*model.CommitResponse{}, nil
	}

	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	// Log the first few requests for debugging
	sampleSize := min(3, len(requests))
	for i := 0; i < sampleSize; i++ {
		c.Log.WithFields(logrus.Fields{
			"index":          i,
			"hash":           requests[i].Hash,
			"message_length": len(requests[i].Message),
			"release_id":     requests[i].ReleaseID,
		}).Debug("Sample commit request")
	}

	// Check for existence of commits to avoid duplicates
	hashes := make([]string, len(requests))
	for i, req := range requests {
		hashes[i] = req.Hash
	}

	// Find existing commits
	var existingCommits []entity.Commit
	if err := tx.Where("hash IN ? AND release_id = ?", hashes, requests[0].ReleaseID).Find(&existingCommits).Error; err != nil {
		c.Log.WithError(err).Warn("Error checking for existing commits")
		// Continue anyway - this is just to avoid duplicates
	}

	// Create a map for quick lookup
	existingMap := make(map[string]bool)
	for _, commit := range existingCommits {
		existingMap[commit.Hash] = true
	}

	// Filter out existing commits
	newRequests := make([]*model.CreateCommitRequest, 0, len(requests))
	for _, req := range requests {
		if !existingMap[req.Hash] {
			newRequests = append(newRequests, req)
		}
	}

	c.Log.WithFields(logrus.Fields{
		"total_commits":    len(requests),
		"existing_commits": len(existingCommits),
		"new_commits":      len(newRequests),
	}).Info("Filtered out existing commits")

	// If all commits already exist, return empty array
	if len(newRequests) == 0 {
		if err := tx.Rollback().Error; err != nil {
			c.Log.WithError(err).Warn("Error rolling back transaction")
		}
		return []*model.CommitResponse{}, nil
	}

	// Create slice of entities for batch insertion
	commits := make([]entity.Commit, len(newRequests))
	for i, req := range newRequests {
		commits[i] = entity.Commit{
			Hash:      req.Hash,
			Message:   req.Message,
			ReleaseID: req.ReleaseID,
		}
	}

	// Use smaller batch sizes to avoid transaction size issues
	batchSize := 50

	// Use CreateInBatches to handle large datasets efficiently
	if err := tx.CreateInBatches(commits, batchSize).Error; err != nil {
		c.Log.WithError(err).Error("Error batch creating commits")

		// Try with individual inserts if batch fails
		c.Log.Info("Batch insert failed, trying individual inserts")

		// Rollback the failed transaction
		tx.Rollback()

		// Start a fresh transaction
		tx = c.DB.WithContext(ctx).Begin()
		defer tx.Rollback()

		// Try inserting one by one
		wg := &sync.WaitGroup{}
		mutex := &sync.Mutex{}
		successCount := 0
		errorCount := 0

		for i, req := range newRequests {
			wg.Add(1)

			go func(index int, request *model.CreateCommitRequest) {
				defer wg.Done()

				commit := entity.Commit{
					Hash:      request.Hash,
					Message:   request.Message,
					ReleaseID: request.ReleaseID,
				}

				err := c.DB.Create(&commit).Error

				mutex.Lock()
				defer mutex.Unlock()

				if err != nil {
					errorCount++
					c.Log.WithFields(logrus.Fields{
						"hash":  commit.Hash[:8] + "...",
						"error": err.Error(),
					}).Warn("Individual commit insert failed")
				} else {
					successCount++
				}
			}(i, req)

			// Limit concurrency
			if i%10 == 0 {
				wg.Wait()
			}
		}

		// Wait for all goroutines
		wg.Wait()

		c.Log.WithFields(logrus.Fields{
			"success_count": successCount,
			"error_count":   errorCount,
		}).Info("Individual insert results")

		// Retrieve all commits for the release to return
		var allCommits []entity.Commit
		if err := c.DB.Where("release_id = ?", newRequests[0].ReleaseID).Find(&allCommits).Error; err != nil {
			c.Log.WithError(err).Error("Failed to retrieve commits after individual inserts")
			return nil, err
		}

		// Create responses for all commits found
		responses := make([]*model.CommitResponse, len(allCommits))
		for i, commit := range allCommits {
			responses[i] = &model.CommitResponse{
				ID:        commit.ID,
				Hash:      commit.Hash,
				Message:   commit.Message,
				ReleaseID: commit.ReleaseID,
			}
		}

		return responses, nil
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.WithError(err).Error("Error committing batch transaction")
		return nil, err
	}

	// Create responses with IDs assigned by database
	responses := make([]*model.CommitResponse, len(commits))
	for i, commit := range commits {
		responses[i] = &model.CommitResponse{
			ID:        commit.ID,
			Hash:      commit.Hash,
			Message:   commit.Message,
			ReleaseID: commit.ReleaseID,
		}
	}

	c.Log.WithField("commit_count", len(responses)).Info("Successfully saved commits")

	return responses, nil
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
