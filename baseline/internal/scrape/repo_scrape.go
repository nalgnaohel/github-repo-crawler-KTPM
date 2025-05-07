package scrape

import (
	"crawler/baseline/internal/model"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gocolly/colly/v2"
)

func CrawlAllRepos() ([]*model.CreateRepoRequest, error) {
	limit := 5000
	log := logrus.New()
	log.Info("Starting to scrape top repositories from gitstar-ranking.com")

	c := colly.NewCollector(
		colly.AllowedDomains("gitstar-ranking.com"),
		colly.MaxDepth(2),
		colly.Async(true),
	)

	repos := make([]*model.CreateRepoRequest, 0, limit)
	paths := make([]string, 0, limit)
	count := 0

	c.OnRequest(func(req *colly.Request) {
		// log.Println("visiting", req.URL.String())
	})

	c.OnHTML("a.list-group-item.paginated_item", func(e *colly.HTMLElement) {
		if count >= limit {
			return
		}

		repoPath := e.Attr("href")
		paths = append(paths, repoPath)
		repoPath = strings.TrimPrefix(repoPath, "/")

		parts := strings.Split(repoPath, "/")
		// fmt.Println(parts)
		repoUser := parts[0]
		repoName := parts[1]

		repo := &model.CreateRepoRequest{
			RepoName: repoName,
			UserName: repoUser,
		}

		repos = append(repos, repo)
		count++

	})

	// Start scraping
	startPage := 1
	maxPages := 50

	for page := startPage; page <= maxPages; page++ {
		pageURL := fmt.Sprintf("https://gitstar-ranking.com/repositories?page=%d", page)
		if err := c.Visit(pageURL); err != nil {
			log.WithError(err).Errorf("Error visiting page %d", page)
		}
	}

	c.Wait()
	// log.Infof("Found %d repositories", len(repos))
	return repos, nil
}
