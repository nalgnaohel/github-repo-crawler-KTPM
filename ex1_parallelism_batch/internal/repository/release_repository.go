package repository

import (
	"crawler/baseline/internal/entity"

	"github.com/sirupsen/logrus"
)

type ReleaseRepository struct {
	Repository[entity.Release]
	Log *logrus.Logger
}

func NewReleaseRepository(log *logrus.Logger) *ReleaseRepository {
	return &ReleaseRepository{
		Log: log,
	}
}
