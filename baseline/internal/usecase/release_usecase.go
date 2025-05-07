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
