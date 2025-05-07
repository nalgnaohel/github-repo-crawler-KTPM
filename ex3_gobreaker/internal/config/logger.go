package config

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// LogConfig holds configuration for different loggers
type LogConfig struct {
	MainLogger    *logrus.Logger
	RepoLogger    *logrus.Logger
	ReleaseLogger *logrus.Logger
	CommitLogger  *logrus.Logger
}

// SetupLoggers initializes all loggers
func SetupLoggers() *LogConfig {
	// Ensure log directory exists
	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic(err)
	}

	// Main application logger
	mainLogger := logrus.New()
	mainLogger.SetFormatter(&logrus.JSONFormatter{})
	mainLogger.SetLevel(logrus.InfoLevel)

	// Repository crawler logger
	repoLogger := createLogger(filepath.Join(logDir, "repo_crawl.log"))

	// Release crawler logger
	releaseLogger := createLogger(filepath.Join(logDir, "release_crawl.log"))
	commitLogger := createLogger(filepath.Join(logDir, "commit_crawl.log"))
	return &LogConfig{
		MainLogger:    mainLogger,
		RepoLogger:    repoLogger,
		ReleaseLogger: releaseLogger,
		CommitLogger:  commitLogger,
	}
}

// createLogger creates a logger with file output
func createLogger(filename string) *logrus.Logger {
	logger := logrus.New()

	// Configure formatter
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Set log level
	logger.SetLevel(logrus.InfoLevel)

	// Create log file
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logger.Fatalf("Failed to open log file %s: %v", filename, err)
	}

	// Multi-writer for console and file
	mw := io.MultiWriter(os.Stdout, file)
	logger.SetOutput(mw)

	return logger
}
