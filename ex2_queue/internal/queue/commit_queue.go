package queue

import (
	"context"
	"crawler/baseline/internal/model"
	"crawler/baseline/internal/usecase"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CommitQueue is the queue component for commit operations
type CommitQueue struct {
	items      []*model.CreateCommitRequest
	mutex      sync.Mutex
	cond       *sync.Cond
	maxSize    int
	metrics    QueueMetrics
	processing int
}

// CommitQueueProcessor handles asynchronous processing of commits
type CommitQueueProcessor struct {
	queue         *CommitQueue
	log           *logrus.Logger
	db            *gorm.DB
	commitUsecase *usecase.CommitUsecase
	ctx           context.Context
	cancel        context.CancelFunc
	workerCount   int
	workerWg      sync.WaitGroup
	batchSize     int
}

// NewCommitQueueProcessor creates a new commit queue processor
func NewCommitQueueProcessor(
	log *logrus.Logger,
	db *gorm.DB,
	commitUsecase *usecase.CommitUsecase,
	maxSize int,
	workerCount int,
	batchSize int,
) *CommitQueueProcessor {
	queue := &CommitQueue{
		items:   make([]*model.CreateCommitRequest, 0),
		maxSize: maxSize,
	}
	queue.cond = sync.NewCond(&queue.mutex)

	ctx, cancel := context.WithCancel(context.Background())

	processor := &CommitQueueProcessor{
		queue:         queue,
		log:           log,
		db:            db,
		commitUsecase: commitUsecase,
		ctx:           ctx,
		cancel:        cancel,
		workerCount:   workerCount,
		batchSize:     batchSize,
	}

	return processor
}

// Start begins processing with worker goroutines
func (p *CommitQueueProcessor) Start() {
	p.log.WithField("worker_count", p.workerCount).Info("Starting commit queue processor")

	for i := 0; i < p.workerCount; i++ {
		p.workerWg.Add(1)
		workerID := i

		go func() {
			defer p.workerWg.Done()
			p.worker(workerID)
		}()
	}

	// Start metrics reporting
	go p.reportMetrics()
}

// Stop terminates all processing
func (p *CommitQueueProcessor) Stop() {
	p.log.Info("Stopping commit queue processor")
	p.cancel()
	p.workerWg.Wait()
	p.log.Info("Commit queue processor stopped")
}

// EnqueueCommit adds a commit to the queue
func (p *CommitQueueProcessor) EnqueueCommit(request *model.CreateCommitRequest) bool {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()

	// Check if queue is full
	if p.queue.maxSize > 0 && len(p.queue.items) >= p.queue.maxSize {
		p.log.Warn("Commit queue is full, applying back pressure")
		return false
	}

	p.queue.items = append(p.queue.items, request)
	p.queue.metrics.EnqueueCount++

	// Update max queue length if needed
	if len(p.queue.items) > p.queue.metrics.MaxQueueLength {
		p.queue.metrics.MaxQueueLength = len(p.queue.items)
	}

	// Signal that items are available
	p.queue.cond.Signal()

	return true
}

// EnqueueItem adds a generic item to the queue
func (p *CommitQueueProcessor) EnqueueItem(item interface{}) bool {
	if commitReq, ok := item.(*model.CreateCommitRequest); ok {
		return p.EnqueueCommit(commitReq)
	}
	p.log.WithField("item_type", item).Warn("Invalid item type for commit queue")
	return false
}

// BatchEnqueueCommits adds multiple commits to the queue
func (p *CommitQueueProcessor) BatchEnqueueCommits(requests []*model.CreateCommitRequest) int {
	enqueued := 0
	for _, req := range requests {
		if p.EnqueueCommit(req) {
			enqueued++
		}
	}
	return enqueued
}

// dequeueCommits gets a batch of commits from the queue
func (p *CommitQueueProcessor) dequeueCommits(maxCount int) []*model.CreateCommitRequest {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()

	// Wait for items if queue is empty
	for len(p.queue.items) == 0 {
		// Check if context is canceled before waiting
		select {
		case <-p.ctx.Done():
			return nil
		default:
			p.queue.cond.Wait()

			// Check context again after being woken up
			select {
			case <-p.ctx.Done():
				return nil
			default:
				// Continue to check if items are available
			}
		}
	}

	// Determine how many items to take
	count := maxCount
	if count > len(p.queue.items) {
		count = len(p.queue.items)
	}

	// Get items and update queue
	items := make([]*model.CreateCommitRequest, count)
	copy(items, p.queue.items[:count])

	p.queue.items = p.queue.items[count:]
	p.queue.metrics.DequeueCount += int64(count)

	// Mark as processing
	p.queue.processing += count

	return items
}

// worker processes items from the queue
func (p *CommitQueueProcessor) worker(workerID int) {
	p.log.WithField("worker_id", workerID).Info("Commit worker started")

	for {
		select {
		case <-p.ctx.Done():
			p.log.WithField("worker_id", workerID).Info("Commit worker stopping")
			return
		default:
			// Get batch of commits
			commits := p.dequeueCommits(p.batchSize)
			if commits == nil || len(commits) == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process commits
			p.processCommits(workerID, commits)

			// Decrement processing count
			p.queue.mutex.Lock()
			p.queue.processing -= len(commits)
			p.queue.mutex.Unlock()
		}
	}
}

// processCommits saves commits to the database
func (p *CommitQueueProcessor) processCommits(workerID int, commits []*model.CreateCommitRequest) {
	if len(commits) == 0 {
		return
	}

	p.log.WithFields(logrus.Fields{
		"worker_id": workerID,
		"count":     len(commits),
	}).Info("Processing batch of commits")

	// Sample data for debugging
	p.log.WithFields(logrus.Fields{
		"first_hash":           commits[0].Hash,
		"first_message_length": len(commits[0].Message),
		"release_id":           commits[0].ReleaseID,
	}).Debug("Sample commit data from batch")

	// Track performance
	startTime := time.Now()

	// Process commits in batch
	responses, err := p.commitUsecase.BatchCreate(context.Background(), commits)

	duration := time.Since(startTime)

	if err != nil {
		p.log.WithFields(logrus.Fields{
			"worker_id":   workerID,
			"error":       err.Error(),
			"duration_ms": duration.Milliseconds(),
			"batch_size":  len(commits),
		}).Error("Error processing batch of commits")

		// Try smaller batches as fallback
		p.log.Info("Trying smaller batches as fallback")

		// Split into smaller batches
		batchSize := 10
		for i := 0; i < len(commits); i += batchSize {
			end := i + batchSize
			if end > len(commits) {
				end = len(commits)
			}

			smallBatch := commits[i:end]
			p.log.WithFields(logrus.Fields{
				"batch_start": i,
				"batch_end":   end,
				"batch_size":  len(smallBatch),
			}).Info("Processing smaller batch")

			batchResp, err := p.commitUsecase.BatchCreate(context.Background(), smallBatch)
			if err != nil {
				p.log.WithError(err).Error("Even smaller batch failed")
			} else {
				p.log.WithField("success_count", len(batchResp)).Info("Smaller batch succeeded")
			}
		}

		return
	}

	p.log.WithFields(logrus.Fields{
		"worker_id":     workerID,
		"success_count": len(responses),
		"duration_ms":   duration.Milliseconds(),
		"batch_size":    len(commits),
	}).Info("Batch processing of commits completed")
}

// GetQueueSize returns the current size of the queue
func (p *CommitQueueProcessor) GetQueueSize() int {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()
	return len(p.queue.items)
}

// GetProcessingCount returns the current number of items being processed
func (p *CommitQueueProcessor) GetProcessingCount() int {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()
	return p.queue.processing
}

// reportMetrics periodically logs queue metrics
func (p *CommitQueueProcessor) reportMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.queue.mutex.Lock()
			metrics := p.queue.metrics
			queueSize := len(p.queue.items)
			processingCount := p.queue.processing
			p.queue.mutex.Unlock()

			p.log.WithFields(logrus.Fields{
				"queue_size":     queueSize,
				"processing":     processingCount,
				"enqueued_total": metrics.EnqueueCount,
				"dequeued_total": metrics.DequeueCount,
				"max_queue_size": metrics.MaxQueueLength,
			}).Info("Commit queue metrics")
		}
	}
}
