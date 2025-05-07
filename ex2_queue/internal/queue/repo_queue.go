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

// RepoQueue is the queue component for repository operations
type RepoQueue struct {
	items      []*model.CreateRepoRequest
	mutex      sync.Mutex
	cond       *sync.Cond
	maxSize    int
	metrics    QueueMetrics
	processing int
}

// RepoQueueProcessor handles asynchronous processing of repositories
type RepoQueueProcessor struct {
	queue       *RepoQueue
	log         *logrus.Logger
	db          *gorm.DB
	repoUsecase *usecase.RepoUsecase
	ctx         context.Context
	cancel      context.CancelFunc
	workerCount int
	workerWg    sync.WaitGroup
	batchSize   int
}

// NewRepoQueueProcessor creates a new repository queue processor
func NewRepoQueueProcessor(
	log *logrus.Logger,
	db *gorm.DB,
	repoUsecase *usecase.RepoUsecase,
	maxSize int,
	workerCount int,
	batchSize int,
) *RepoQueueProcessor {
	queue := &RepoQueue{
		items:   make([]*model.CreateRepoRequest, 0),
		maxSize: maxSize,
	}
	queue.cond = sync.NewCond(&queue.mutex)

	ctx, cancel := context.WithCancel(context.Background())

	processor := &RepoQueueProcessor{
		queue:       queue,
		log:         log,
		db:          db,
		repoUsecase: repoUsecase,
		ctx:         ctx,
		cancel:      cancel,
		workerCount: workerCount,
		batchSize:   batchSize,
	}

	return processor
}

// Start begins processing with worker goroutines
func (p *RepoQueueProcessor) Start() {
	p.log.WithField("worker_count", p.workerCount).Info("Starting repository queue processor")

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
func (p *RepoQueueProcessor) Stop() {
	p.log.Info("Stopping repository queue processor")
	p.cancel()
	p.workerWg.Wait()
	p.log.Info("Repository queue processor stopped")
}

// EnqueueRepo adds a repository to the queue
func (p *RepoQueueProcessor) EnqueueRepo(request *model.CreateRepoRequest) bool {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()

	// Check if queue is full
	if p.queue.maxSize > 0 && len(p.queue.items) >= p.queue.maxSize {
		p.log.Warn("Repository queue is full, applying back pressure")
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
func (p *RepoQueueProcessor) EnqueueItem(item interface{}) bool {
	if repoReq, ok := item.(*model.CreateRepoRequest); ok {
		return p.EnqueueRepo(repoReq)
	}
	p.log.WithField("item_type", item).Warn("Invalid item type for repository queue")
	return false
}

// BatchEnqueueRepos adds multiple repositories to the queue
func (p *RepoQueueProcessor) BatchEnqueueRepos(requests []*model.CreateRepoRequest) int {
	enqueued := 0
	for _, req := range requests {
		if p.EnqueueRepo(req) {
			enqueued++
		}
	}
	return enqueued
}

// dequeueRepos gets a batch of repositories from the queue
func (p *RepoQueueProcessor) dequeueRepos(maxCount int) []*model.CreateRepoRequest {
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
	items := make([]*model.CreateRepoRequest, count)
	copy(items, p.queue.items[:count])

	p.queue.items = p.queue.items[count:]
	p.queue.metrics.DequeueCount += int64(count)

	// Mark as processing
	p.queue.processing += count

	return items
}

// worker processes items from the queue
func (p *RepoQueueProcessor) worker(workerID int) {
	p.log.WithField("worker_id", workerID).Info("Repository worker started")

	for {
		select {
		case <-p.ctx.Done():
			p.log.WithField("worker_id", workerID).Info("Repository worker stopping")
			return
		default:
			// Get batch of repositories
			repos := p.dequeueRepos(p.batchSize)
			if repos == nil || len(repos) == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process repositories
			p.processRepos(workerID, repos)

			// Decrement processing count
			p.queue.mutex.Lock()
			p.queue.processing -= len(repos)
			p.queue.mutex.Unlock()
		}
	}
}

// processRepos saves repositories to the database
func (p *RepoQueueProcessor) processRepos(workerID int, repos []*model.CreateRepoRequest) {
	if len(repos) == 0 {
		return
	}

	p.log.WithFields(logrus.Fields{
		"worker_id": workerID,
		"count":     len(repos),
	}).Debug("Processing batch of repositories")

	// Track performance
	startTime := time.Now()

	// Process repositories in batch
	responses, err := p.repoUsecase.BatchCreate(context.Background(), repos)

	duration := time.Since(startTime)

	if err != nil {
		p.log.WithFields(logrus.Fields{
			"worker_id":   workerID,
			"error":       err.Error(),
			"duration_ms": duration.Milliseconds(),
			"batch_size":  len(repos),
		}).Error("Error processing batch of repositories")
		return
	}

	p.log.WithFields(logrus.Fields{
		"worker_id":     workerID,
		"success_count": len(responses),
		"duration_ms":   duration.Milliseconds(),
		"batch_size":    len(repos),
	}).Info("Batch processing of repositories completed")
}

// GetQueueSize returns the current size of the queue
func (p *RepoQueueProcessor) GetQueueSize() int {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()
	return len(p.queue.items)
}

// GetProcessingCount returns the current number of items being processed
func (p *RepoQueueProcessor) GetProcessingCount() int {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()
	return p.queue.processing
}

// reportMetrics periodically logs queue metrics
func (p *RepoQueueProcessor) reportMetrics() {
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
			}).Info("Repository queue metrics")
		}
	}
}
