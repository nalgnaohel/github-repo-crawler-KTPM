package repository

import (
	"crawler/baseline/internal/entity"

	"github.com/sirupsen/logrus"
)

type CommitRepository struct {
	Repository[entity.Commit]
	Log *logrus.Logger
}

func NewCommitRepository(log *logrus.Logger) *CommitRepository {
	return &CommitRepository{
		Log: log,
	}
}
