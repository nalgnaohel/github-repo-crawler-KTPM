package repository

import (
	"crawler/baseline/internal/entity"

	"github.com/sirupsen/logrus"
)

type RepoRepository struct {
	Repository[entity.Repository]
	Log *logrus.Logger
}

func NewRepoRepository(log *logrus.Logger) *RepoRepository {
	return &RepoRepository{
		Log: log,
	}
}
