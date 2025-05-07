package usecase

import (
	"context"
	"crawler/baseline/internal/entity"
	"crawler/baseline/internal/model"
	"crawler/baseline/internal/repository"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type RepoUsecase struct {
	DB             *gorm.DB
	Log            *logrus.Logger
	RepoRepository *repository.RepoRepository
}

func NewRepoUsecase(db *gorm.DB, log *logrus.Logger,
	repoRepo *repository.RepoRepository) *RepoUsecase {
	return &RepoUsecase{
		DB:             db,
		Log:            log,
		RepoRepository: repoRepo,
	}
}

func (r *RepoUsecase) Create(ctx context.Context, request *model.CreateRepoRequest) (*model.RepoResponse, error) {
	tx := r.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	// Create repository entity that matches your schema
	repo := &entity.Repository{
		RepoName: request.RepoName,
		UserName: request.UserName,
	}

	if err := r.RepoRepository.Create(tx, repo); err != nil {
		r.Log.WithError(err).Error("error creating repository")
		return nil, nil
	}

	if err := tx.Commit().Error; err != nil {
		r.Log.WithError(err).Error("error committing transaction")
		return nil, nil
	}

	return &model.RepoResponse{
		ID:       repo.ID,
		RepoName: repo.RepoName,
		UserName: repo.UserName,
	}, nil
}
