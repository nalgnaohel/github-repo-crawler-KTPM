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

func (r *RepoUsecase) BatchCreate(ctx context.Context, requests []*model.CreateRepoRequest) ([]*model.RepoResponse, error) {
	if len(requests) == 0 {
		return []*model.RepoResponse{}, nil
	}

	tx := r.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	// Create slice of entities for batch insertion
	repos := make([]entity.Repository, len(requests))
	for i, req := range requests {
		repos[i] = entity.Repository{
			RepoName: req.RepoName,
			UserName: req.UserName,
		}
	}

	// Perform batch insert
	if err := tx.CreateInBatches(repos, 100).Error; err != nil {
		r.Log.WithError(err).Error("error batch creating repositories")
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		r.Log.WithError(err).Error("error committing batch transaction")
		return nil, err
	}

	// Create responses with IDs assigned by database
	responses := make([]*model.RepoResponse, len(repos))
	for i, repo := range repos {
		responses[i] = &model.RepoResponse{
			ID:       repo.ID,
			RepoName: repo.RepoName,
			UserName: repo.UserName,
		}
	}

	return responses, nil
}
