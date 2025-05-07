package main

import (
	"crawler/baseline/internal/config"
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("Hello, World!")
	viperConfig := config.NewViper()
	logConfig := config.NewLogger(viperConfig)
	dbConfig := config.NewDatabase(viperConfig, logConfig)

	r := config.Bootstrap(&config.BootstrapConfig{
		DB:     dbConfig,
		Log:    logConfig,
		Config: viperConfig,
	})

	http.ListenAndServe(":8080", r)

}

// func main() {
// 	scrape.CrawlCommit("opencv", "opencv", "4.11.0")
// }
