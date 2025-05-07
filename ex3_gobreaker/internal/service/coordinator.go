package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"crawler/baseline/internal/utils"
)

// CrawlingCoordinator orchestrates the crawling operations with circuit breaker protection
type CrawlingCoordinator struct {
	baseURL   string
	repoCB    *utils.CircuitBreakerWrapper
	releaseCB *utils.CircuitBreakerWrapper
	commitCB  *utils.CircuitBreakerWrapper

	// Cache for comparing responses
	repoCache    interface{}
	releaseCache interface{}
	commitCache  interface{}

	// Track consecutive no-change responses to stop calling stable endpoints
	repoNoChangeCount    int
	releaseNoChangeCount int
	commitNoChangeCount  int

	// Flags to track if endpoints are paused due to stability
	repoPaused    bool
	releasePaused bool
	commitPaused  bool

	// Threshold for number of no-changes before pausing
	stabilityThreshold int

	cacheMutex sync.RWMutex
	client     *http.Client
}

// NewCrawlingCoordinator creates a new crawling coordinator
func NewCrawlingCoordinator(baseURL string) *CrawlingCoordinator {
	return &CrawlingCoordinator{
		baseURL:            baseURL,
		repoCB:             utils.NewCircuitBreaker("repo-crawler"),
		releaseCB:          utils.NewCircuitBreaker("release-crawler"),
		commitCB:           utils.NewCircuitBreaker("commit-crawler"),
		client:             &http.Client{Timeout: 30 * time.Second},
		stabilityThreshold: 3, // Stop calling after 3 consecutive no-change responses
	}
}

// CrawlRepos crawls repositories with circuit breaker protection
func (c *CrawlingCoordinator) CrawlRepos() (interface{}, error) {
	result, err := c.repoCB.Execute(func() (interface{}, error) {
		resp, err := c.client.Get(fmt.Sprintf("%s/repos/crawl", c.baseURL))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to crawl repos: status %d", resp.StatusCode)
		}

		var data interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, err
		}

		return data, nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// CrawlReleases crawls releases with circuit breaker protection
func (c *CrawlingCoordinator) CrawlReleases() (interface{}, error) {
	result, err := c.releaseCB.Execute(func() (interface{}, error) {
		resp, err := c.client.Get(fmt.Sprintf("%s/releases/crawl", c.baseURL))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to crawl releases: status %d", resp.StatusCode)
		}

		var data interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, err
		}

		return data, nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// CrawlCommits crawls commits with circuit breaker protection
func (c *CrawlingCoordinator) CrawlCommits() (interface{}, error) {
	result, err := c.commitCB.Execute(func() (interface{}, error) {
		resp, err := c.client.Get(fmt.Sprintf("%s/commits/crawl", c.baseURL))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to crawl commits: status %d", resp.StatusCode)
		}

		var data interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, err
		}

		return data, nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// hasDataChanged compares previous and current data to detect changes
func (c *CrawlingCoordinator) hasDataChanged(previous, current interface{}) bool {
	if previous == nil {
		return true
	}

	// Convert both to JSON for deep comparison
	prevJSON, _ := json.Marshal(previous)
	currJSON, _ := json.Marshal(current)

	// Compare JSON strings
	return string(prevJSON) != string(currJSON)
}

// CrawlAll orchestrates the crawling of all data with interdependencies
func (c *CrawlingCoordinator) CrawlAll() {
	var wg sync.WaitGroup
	repoChanged := false
	releaseChanged := false

	// Step 1: Crawl repositories only if we haven't successfully done so yet or previous attempt failed
	c.cacheMutex.RLock()
	repoNeedsCall := c.repoCache == nil // Only call if we don't have repo data yet
	c.cacheMutex.RUnlock()

	if repoNeedsCall {
		wg.Add(1)
		go func() {
			defer wg.Done()

			log.Println("Starting repository crawling (one-time)...")
			repoData, err := c.CrawlRepos()
			if err != nil {
				log.Printf("Error crawling repositories: %v", err)
				return
			}

			log.Println("Repository data successfully fetched")
			c.cacheMutex.Lock()
			c.repoCache = repoData
			repoChanged = true

			// Initial repo data fetch should trigger release check
			c.releasePaused = false
			c.releaseNoChangeCount = 0
			c.cacheMutex.Unlock()
		}()

		wg.Wait()
	} else {
		log.Println("Repository data already fetched, skipping repo API call")
	}

	// Step 2: If repos changed or no release data yet, crawl releases
	wg.Add(1)
	go func() {
		defer wg.Done()

		c.cacheMutex.RLock()
		shouldCrawl := (repoChanged || c.releaseCache == nil) && !c.releasePaused
		c.cacheMutex.RUnlock()

		if !shouldCrawl {
			if c.releasePaused {
				log.Println("Release API is stable, skipping call")
			} else {
				log.Println("Skipping release crawling, no repo changes")
			}
			return
		}

		log.Println("Starting release crawling...")
		releaseData, err := c.CrawlReleases()
		if err != nil {
			log.Printf("Error crawling releases: %v", err)
			return
		}

		c.cacheMutex.RLock()
		prevData := c.releaseCache
		c.cacheMutex.RUnlock()

		if c.hasDataChanged(prevData, releaseData) {
			log.Println("Release data has changed")
			c.cacheMutex.Lock()
			c.releaseCache = releaseData
			c.releaseNoChangeCount = 0
			releaseChanged = true

			// When releases change, unpause commit API
			c.commitPaused = false
			c.commitNoChangeCount = 0
			c.cacheMutex.Unlock()
		} else {
			log.Println("No changes in release data")
			c.cacheMutex.Lock()
			c.releaseNoChangeCount++

			// Check if we should pause this endpoint
			if c.releaseNoChangeCount >= c.stabilityThreshold {
				c.releasePaused = true
				log.Println("Release API has been stable for multiple checks, pausing calls")
			}
			c.cacheMutex.Unlock()
		}
	}()

	wg.Wait()

	// Step 3: If releases changed or no commit data yet, crawl commits
	wg.Add(1)
	go func() {
		defer wg.Done()

		c.cacheMutex.RLock()
		shouldCrawl := (releaseChanged || c.commitCache == nil) && !c.commitPaused
		c.cacheMutex.RUnlock()

		if !shouldCrawl {
			if c.commitPaused {
				log.Println("Commit API is stable, skipping call")
			} else {
				log.Println("Skipping commit crawling, no release changes")
			}
			return
		}

		log.Println("Starting commit crawling...")
		commitData, err := c.CrawlCommits()
		if err != nil {
			log.Printf("Error crawling commits: %v", err)
			return
		}

		c.cacheMutex.RLock()
		prevData := c.commitCache
		c.cacheMutex.RUnlock()

		if c.hasDataChanged(prevData, commitData) {
			log.Println("Commit data has changed")
			c.cacheMutex.Lock()
			c.commitCache = commitData
			c.commitNoChangeCount = 0
			c.cacheMutex.Unlock()
		} else {
			log.Println("No changes in commit data")
			c.cacheMutex.Lock()
			c.commitNoChangeCount++

			// Check if we should pause this endpoint
			if c.commitNoChangeCount >= c.stabilityThreshold {
				c.commitPaused = true
				log.Println("Commit API has been stable for multiple checks, pausing calls")
			}
			c.cacheMutex.Unlock()
		}
	}()

	wg.Wait()

	// Check status of APIs - for logging purposes
	c.cacheMutex.RLock()
	releaseAndCommitPaused := c.releasePaused && c.commitPaused
	c.cacheMutex.RUnlock()

	if releaseAndCommitPaused {
		log.Println("All dependent APIs are stable, monitoring for changes")
	} else {
		log.Println("Crawling cycle completed")
	}
}

// ForceReactivateAll forcibly reactivates all API endpoints
func (c *CrawlingCoordinator) ForceReactivateAll() {
	c.cacheMutex.Lock()
	c.repoCache = nil // Force repo to be fetched again
	c.releasePaused = false
	c.commitPaused = false
	c.repoNoChangeCount = 0
	c.releaseNoChangeCount = 0
	c.commitNoChangeCount = 0
	c.cacheMutex.Unlock()
	log.Println("Forcibly reactivated all API endpoints")
}

// SetStabilityThreshold sets the number of consecutive no-change responses before pausing an endpoint
func (c *CrawlingCoordinator) SetStabilityThreshold(threshold int) {
	if threshold < 1 {
		threshold = 1
	}
	c.cacheMutex.Lock()
	c.stabilityThreshold = threshold
	c.cacheMutex.Unlock()
	log.Printf("Stability threshold set to %d consecutive no-change responses", threshold)
}

// StartPeriodicCrawling continuously monitors for changes and crawls data
func (c *CrawlingCoordinator) StartPeriodicCrawling(interval time.Duration, stopChan <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.CrawlAll()
		case <-stopChan:
			log.Println("Stopping periodic crawling")
			return
		}
	}
}
