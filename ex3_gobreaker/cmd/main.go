package main

import (
	"crawler/baseline/internal/config"
	"crawler/baseline/internal/service"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func startCircuitBreakerCoordinator(baseURL string, interval int) {
	log.Printf("Starting circuit breaker coordinator with interval: %d seconds", interval)

	// Create coordinator with circuit breaker protection
	coordinator := service.NewCrawlingCoordinator(baseURL)

	// Setup signal handling for graceful shutdown
	stopChan := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received for coordinator")
		close(stopChan)
	}()

	// Initial crawl to populate caches
	log.Println("Running initial data crawl...")
	coordinator.CrawlAll()

	// Start periodic crawling
	log.Printf("Starting periodic monitoring every %d seconds", interval)
	coordinator.StartPeriodicCrawling(time.Duration(interval)*time.Second, stopChan)
}

func main() {
	fmt.Println("Hello, World!")
	viperConfig := config.NewViper()
	logConfig := config.NewLogger(viperConfig)
	dbConfig := config.NewDatabase(viperConfig, logConfig)
	collyConfig := config.NewColly(viperConfig, logConfig)

	// Start circuit breaker coordinator in the background
	go startCircuitBreakerCoordinator("http://localhost:8081/api", 60)

	r := config.Bootstrap(&config.BootstrapConfig{
		DB:     dbConfig,
		Log:    logConfig,
		Config: viperConfig,
		Colly:  collyConfig,
	})

	fmt.Println("Starting HTTP server on :8081")
	http.ListenAndServe(":8081", r)
}

// func main() {
// 	scrape.CrawlCommit("opencv", "opencv", "4.11.0")
// }
