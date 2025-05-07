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
	collyConfig := config.NewColly(viperConfig, logConfig)

	r := config.Bootstrap(&config.BootstrapConfig{
		DB:     dbConfig,
		Log:    logConfig,
		Config: viperConfig,
		Colly:  collyConfig,
	})

	http.ListenAndServe(":8081", r)

}

// func main() {
// 	scrape.CrawlCommit("opencv", "opencv", "4.11.0")
// }
