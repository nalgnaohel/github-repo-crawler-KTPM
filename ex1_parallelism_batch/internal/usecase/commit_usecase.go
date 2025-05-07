package usecase

import (
	"context"
	"crawler/baseline/internal/entity"
	"crawler/baseline/internal/model"
	"crawler/baseline/internal/repository"

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

	// Create commit entity
	commit := &entity.Commit{
		Hash:      request.Hash,
		Message:   request.Message,
		ReleaseID: request.ReleaseID,
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
	var commitEntities []entity.Commit

	if err := c.DB.Where("release_id = ?", releaseID).Find(&commitEntities).Error; err != nil {
		c.Log.WithError(err).Error("error fetching commits for release")
		return nil, err
	}

	// Convert entities to response models
	commits := make([]*model.CommitResponse, len(commitEntities))
	for i, entity := range commitEntities {
		commits[i] = &model.CommitResponse{
			ID:        entity.ID,
			Hash:      entity.Hash,
			Message:   entity.Message,
			ReleaseID: entity.ReleaseID,
		}
	}

	return commits, nil
}

// BatchCreate inserts multiple commits in a single transaction
func (c *CommitUsecase) BatchCreate(ctx context.Context, requests []*model.CreateCommitRequest) ([]*model.CommitResponse, error) {
	if len(requests) == 0 {
		return []*model.CommitResponse{}, nil
	}

	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	// Create slice of entities for batch insertion
	commits := make([]entity.Commit, len(requests))
	for i, req := range requests {
		commits[i] = entity.Commit{
			Hash:      req.Hash,
			Message:   req.Message,
			ReleaseID: req.ReleaseID,
		}
	}

	// Use CreateInBatches to handle large datasets efficiently
	// The second parameter (100) is the batch size
	if err := tx.CreateInBatches(commits, 100).Error; err != nil {
		c.Log.WithError(err).Error("error batch creating commits")
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.WithError(err).Error("error committing batch transaction")
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

	return responses, nil
}
