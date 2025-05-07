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

// ReleaseQueue is the queue component for release operations
type ReleaseQueue struct {
	items      []*model.CreateReleaseRequest
	mutex      sync.Mutex // Changed from RWMutex to regular Mutex
	cond       *sync.Cond
	maxSize    int
	metrics    QueueMetrics
	processing int
}

// ReleaseQueueProcessor handles asynchronous processing of releases
type ReleaseQueueProcessor struct {
	queue          *ReleaseQueue
	log            *logrus.Logger
	db             *gorm.DB
	releaseUsecase *usecase.ReleaseUsecase
	ctx            context.Context
	cancel         context.CancelFunc
	workerCount    int
	workerWg       sync.WaitGroup
	batchSize      int
}

// QueueMetrics tracks metrics for queue operations
type QueueMetrics struct {
	EnqueueCount   int64
	DequeueCount   int64
	ProcessingTime time.Duration
	WaitTime       time.Duration
	MaxQueueLength int
}

// NewReleaseQueueProcessor creates a new release queue processor
func NewReleaseQueueProcessor(
	log *logrus.Logger,
	db *gorm.DB,
	releaseUsecase *usecase.ReleaseUsecase,
	maxSize int,
	workerCount int,
	batchSize int,
) *ReleaseQueueProcessor {
	queue := &ReleaseQueue{
		items:   make([]*model.CreateReleaseRequest, 0),
		maxSize: maxSize,
	}
	queue.cond = sync.NewCond(&queue.mutex) // Use the mutex directly

	ctx, cancel := context.WithCancel(context.Background())

	processor := &ReleaseQueueProcessor{
		queue:          queue,
		log:            log,
		db:             db,
		releaseUsecase: releaseUsecase,
		ctx:            ctx,
		cancel:         cancel,
		workerCount:    workerCount,
		batchSize:      batchSize,
	}

	return processor
}

// Start begins processing with worker goroutines
func (p *ReleaseQueueProcessor) Start() {
	p.log.WithField("worker_count", p.workerCount).Info("Starting release queue processor")

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
func (p *ReleaseQueueProcessor) Stop() {
	p.log.Info("Stopping release queue processor")
	p.cancel()
	p.workerWg.Wait()
	p.log.Info("Release queue processor stopped")
}

// EnqueueRelease adds a release to the queue
func (p *ReleaseQueueProcessor) EnqueueRelease(request *model.CreateReleaseRequest) bool {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()

	// Check if queue is full
	if p.queue.maxSize > 0 && len(p.queue.items) >= p.queue.maxSize {
		p.log.Warn("Release queue is full, applying back pressure")
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
func (p *ReleaseQueueProcessor) EnqueueItem(item interface{}) bool {
	if releaseReq, ok := item.(*model.CreateReleaseRequest); ok {
		return p.EnqueueRelease(releaseReq)
	}
	p.log.WithField("item_type", item).Warn("Invalid item type for release queue")
	return false
}

// BatchEnqueueReleases adds multiple releases to the queue
func (p *ReleaseQueueProcessor) BatchEnqueueReleases(requests []*model.CreateReleaseRequest) int {
	enqueued := 0
	for _, req := range requests {
		if p.EnqueueRelease(req) {
			enqueued++
		}
	}
	return enqueued
}

// dequeueReleases gets a batch of releases from the queue
func (p *ReleaseQueueProcessor) dequeueReleases(maxCount int) []*model.CreateReleaseRequest {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()

	// Wait for items if queue is empty - FIXED: proper condition variable usage
	for len(p.queue.items) == 0 {
		// Check if context is canceled before waiting
		select {
		case <-p.ctx.Done():
			return nil
		default:
			// Wait for signal - this will atomically unlock the mutex while waiting
			// and reacquire it when woken up
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

	// At this point we have the lock and there are items in the queue

	// Determine how many items to take
	count := maxCount
	if count > len(p.queue.items) {
		count = len(p.queue.items)
	}

	// Get items and update queue
	items := make([]*model.CreateReleaseRequest, count)
	copy(items, p.queue.items[:count])

	p.queue.items = p.queue.items[count:]
	p.queue.metrics.DequeueCount += int64(count)

	// Mark as processing
	p.queue.processing += count

	return items
}

// worker processes items from the queue
func (p *ReleaseQueueProcessor) worker(workerID int) {
	p.log.WithField("worker_id", workerID).Info("Release worker started")

	for {
		select {
		case <-p.ctx.Done():
			p.log.WithField("worker_id", workerID).Info("Release worker stopping")
			return
		default:
			// Get batch of releases
			releases := p.dequeueReleases(p.batchSize)
			if releases == nil || len(releases) == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process releases
			p.processReleases(workerID, releases)

			// Decrement processing count
			p.queue.mutex.Lock()
			p.queue.processing -= len(releases)
			p.queue.mutex.Unlock()
		}
	}
}

// processReleases saves releases to the database
func (p *ReleaseQueueProcessor) processReleases(workerID int, releases []*model.CreateReleaseRequest) {
	if len(releases) == 0 {
		return
	}

	p.log.WithFields(logrus.Fields{
		"worker_id": workerID,
		"count":     len(releases),
	}).Debug("Processing batch of releases")

	// Track performance
	startTime := time.Now()

	// Use batch create for better performance
	responses, err := p.releaseUsecase.BatchCreate(context.Background(), releases)

	duration := time.Since(startTime)

	if err != nil {
		p.log.WithFields(logrus.Fields{
			"worker_id":   workerID,
			"error":       err.Error(),
			"duration_ms": duration.Milliseconds(),
			"batch_size":  len(releases),
		}).Error("Error processing batch of releases")
		return
	}

	p.log.WithFields(logrus.Fields{
		"worker_id":     workerID,
		"success_count": len(responses),
		"duration_ms":   duration.Milliseconds(),
		"batch_size":    len(releases),
	}).Info("Batch processing of releases completed")
}

// GetQueueSize returns the current size of the queue
func (p *ReleaseQueueProcessor) GetQueueSize() int {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()
	return len(p.queue.items)
}

// GetProcessingCount returns the current number of items being processed
func (p *ReleaseQueueProcessor) GetProcessingCount() int {
	p.queue.mutex.Lock()
	defer p.queue.mutex.Unlock()
	return p.queue.processing
}

// reportMetrics periodically logs queue metrics
func (p *ReleaseQueueProcessor) reportMetrics() {
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
			}).Info("Release queue metrics")
		}
	}
}
