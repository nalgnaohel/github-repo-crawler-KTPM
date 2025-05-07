package utils

import (
	"time"

	"github.com/sirupsen/logrus"
)

// OperationTimer tracks execution time for different operations
type OperationTimer struct {
	Name          string
	StartTime     time.Time
	ScrapeTime    time.Duration
	DatabaseTime  time.Duration
	TotalTime     time.Duration
	Log           *logrus.Logger
	OperationType string // "repo" or "release"
}

// NewOperationTimer creates a new timer with the given name
func NewOperationTimer(name string, operationType string, log *logrus.Logger) *OperationTimer {
	return &OperationTimer{
		Name:          name,
		StartTime:     time.Now(),
		OperationType: operationType,
		Log:           log,
	}
}

// StartScraping marks the beginning of the scraping phase
func (t *OperationTimer) StartScraping() time.Time {
	t.StartTime = time.Now()
	return t.StartTime
}

// EndScraping marks the end of the scraping phase and records the duration
func (t *OperationTimer) EndScraping() time.Duration {
	t.ScrapeTime = time.Since(t.StartTime)
	t.logTiming("Scraping", t.ScrapeTime)
	return t.ScrapeTime
}

// StartDatabase marks the beginning of the database operation phase
func (t *OperationTimer) StartDatabase() time.Time {
	return time.Now()
}

// EndDatabase marks the end of the database operation phase and records the duration
func (t *OperationTimer) EndDatabase(startTime time.Time) time.Duration {
	t.DatabaseTime = time.Since(startTime)
	t.logTiming("Database", t.DatabaseTime)
	return t.DatabaseTime
}

// End marks the completion of the entire operation and logs total time
func (t *OperationTimer) End() {
	t.TotalTime = time.Since(t.StartTime)
	t.logTiming("Total", t.TotalTime)

	// Log overall summary
	t.Log.WithFields(logrus.Fields{
		"operation":     t.Name,
		"scrape_time":   t.ScrapeTime.String(),
		"database_time": t.DatabaseTime.String(),
		"total_time":    t.TotalTime.String(),
		"type":          t.OperationType,
	}).Info("Operation completed")
}

// logTiming logs the timing information for a specific phase
func (t *OperationTimer) logTiming(phase string, duration time.Duration) {
	t.Log.WithFields(logrus.Fields{
		"operation": t.Name,
		"phase":     phase,
		"duration":  duration.String(),
		"ms":        duration.Milliseconds(),
		"type":      t.OperationType,
	}).Info("Timing information")
}
