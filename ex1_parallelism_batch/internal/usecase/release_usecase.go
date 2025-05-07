package usecase

import (
	"context"
	"crawler/baseline/internal/entity"
	"crawler/baseline/internal/model"
	"crawler/baseline/internal/repository"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ReleaseUsecase struct {
	DB                *gorm.DB
	Log               *logrus.Logger
	ReleaseRepository *repository.ReleaseRepository
}

func NewReleaseUsecase(db *gorm.DB, log *logrus.Logger,
	releaseRepo *repository.ReleaseRepository) *ReleaseUsecase {
	return &ReleaseUsecase{
		DB:                db,
		Log:               log,
		ReleaseRepository: releaseRepo,
	}
}

func (r *ReleaseUsecase) Create(ctx context.Context, request *model.CreateReleaseRequest) (*model.ReleaseResponse, error) {
	tx := r.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	release := &entity.Release{
		TagName: request.TagName,
		Content: request.Content,
		RepoID:  request.RepoID,
	}
	if err := r.ReleaseRepository.Create(tx, release); err != nil {
		r.Log.WithError(err).Error("error creating release")
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		r.Log.WithError(err).Error("error committing transaction")
		return nil, err
	}
	return &model.ReleaseResponse{
		ID:      release.ID,
		Content: release.Content,
		RepoID:  release.RepoID,
	}, nil
}

func (r *ReleaseUsecase) BatchCreate(ctx context.Context, requests []*model.CreateReleaseRequest) ([]*model.ReleaseResponse, error) {
	if len(requests) == 0 {
		return []*model.ReleaseResponse{}, nil
	}

	tx := r.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	// Create slice of entities for batch insertion
	releases := make([]entity.Release, len(requests))
	for i, req := range requests {
		releases[i] = entity.Release{
			TagName: req.TagName,
			Content: req.Content,
			RepoID:  req.RepoID,
		}
	}

	// Perform batch insert with chunks of 100
	if err := tx.CreateInBatches(releases, 100).Error; err != nil {
		r.Log.WithError(err).Error("error batch creating releases")
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		r.Log.WithError(err).Error("error committing batch transaction")
		return nil, err
	}

	// Create responses with IDs assigned by database
	responses := make([]*model.ReleaseResponse, len(releases))
	for i, release := range releases {
		responses[i] = &model.ReleaseResponse{
			ID:      release.ID,
			TagName: release.TagName,
			Content: release.Content,
			RepoID:  release.RepoID,
		}
	}

	return responses, nil
}
