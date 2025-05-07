package scrape

import (
	"crawler/baseline/internal/model"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gocolly/colly/v2"
)

type RepoScrape struct {
	Log   *logrus.Logger
	Colly *colly.Collector
}

func NewRepoScrape(log *logrus.Logger, colly *colly.Collector) *RepoScrape {
	return &RepoScrape{
		Log:   log,
		Colly: colly,
	}
}

func (s *RepoScrape) CrawlAllRepos() ([]*model.CreateRepoRequest, error) {
	limit := 5000
	s.Log.Info("Starting to scrape top repositories from gitstar-ranking.com")

	repos := make([]*model.CreateRepoRequest, 0, limit)
	paths := make([]string, 0, limit)
	count := 0

	s.Colly.OnRequest(func(req *colly.Request) {
		// log.Println("visiting", req.URL.String())
	})

	s.Colly.OnHTML("a.list-group-item.paginated_item", func(e *colly.HTMLElement) {
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
		if err := s.Colly.Visit(pageURL); err != nil {
			s.Log.WithError(err).Errorf("Error visiting page %d", page)
		}
	}

	s.Colly.Wait()
	// log.Infof("Found %d repositories", len(repos))
	return repos, nil
}
