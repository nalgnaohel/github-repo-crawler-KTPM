package queue

import (
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// QueueConfig holds configuration for the queue system
type QueueConfig struct {
	MaxSize int
	Workers struct {
		Repo    int
		Release int
		Commit  int
	}
	BatchSize struct {
		Min int
		Max int
	}
	Retry struct {
		MaxAttempts int
		DelayMs     int
	}
}

// NewQueueConfig creates a queue configuration from viper
func NewQueueConfig(v *viper.Viper, log *logrus.Logger) *QueueConfig {
	// Default values
	config := &QueueConfig{
		MaxSize: 10000,
	}

	// Default to number of CPUs for worker count
	cpuCount := runtime.NumCPU()
	if cpuCount < 2 {
		cpuCount = 2
	}

	config.Workers.Repo = cpuCount
	config.Workers.Release = cpuCount
	config.Workers.Commit = cpuCount
	config.BatchSize.Min = 5
	config.BatchSize.Max = 100
	config.Retry.MaxAttempts = 3
	config.Retry.DelayMs = 1000

	// Try to read from config
	if err := v.UnmarshalKey("queue", config); err != nil {
		log.WithError(err).Warn("Failed to parse queue configuration, using defaults")
	}

	// Validate and apply sensible defaults
	if config.MaxSize <= 0 {
		log.Warn("Invalid queue max_size, using default of 10000")
		config.MaxSize = 10000
	}

	if config.Workers.Repo <= 0 {
		log.Warn("Invalid repo workers count, using CPUs")
		config.Workers.Repo = cpuCount
	}

	if config.Workers.Release <= 0 {
		log.Warn("Invalid release workers count, using CPUs")
		config.Workers.Release = cpuCount
	}

	if config.Workers.Commit <= 0 {
		log.Warn("Invalid commit workers count, using CPUs")
		config.Workers.Commit = cpuCount
	}

	if config.BatchSize.Min <= 0 {
		config.BatchSize.Min = 5
	}

	if config.BatchSize.Max <= config.BatchSize.Min {
		config.BatchSize.Max = config.BatchSize.Min * 10
	}

	log.WithFields(logrus.Fields{
		"max_size":        config.MaxSize,
		"repo_workers":    config.Workers.Repo,
		"release_workers": config.Workers.Release,
		"commit_workers":  config.Workers.Commit,
		"batch_size_min":  config.BatchSize.Min,
		"batch_size_max":  config.BatchSize.Max,
	}).Info("Queue configuration loaded")

	return config
}
