package scrape

import (
	"crawler/baseline/internal/utils"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
)

type ReleaseScrape struct {
	Log   *logrus.Logger
	Colly *colly.Collector
}

func NewReleaseScrape(log *logrus.Logger, colly *colly.Collector) *ReleaseScrape {
	return &ReleaseScrape{
		Log:   log,
		Colly: colly,
	}
}

func (s *ReleaseScrape) CrawlRelease(repoOwner string, repoName string, releaseTag string) string {
	releaseURL := "https://github.com/" + repoOwner + "/" + repoName + "/releases/tag/" + releaseTag
	// s.Log.Info("Starting to scrape release: ", releaseURL)
	s.Colly.OnRequest(func(req *colly.Request) {
		// s.Log.Info("visiting: ", releaseURL)
	})
	contentData := ""
	s.Colly.OnHTML("div.Box-body", func(e *colly.HTMLElement) {
		e.DOM.Find("div.markdown-body.my-3").Each(func(i int, s *goquery.Selection) {
			contentData += s.Text() + "\n"
		})
	})

	err := s.Colly.Visit(releaseURL)
	if err != nil {
		s.Log.Error("Error visiting release URL: ", err)
		return ""
	}
	s.Log.Info("Scraping completed for release: ", releaseTag)
	// s.Log.Info("Content: ", contentData)
	return contentData
}

func (s *ReleaseScrape) CrawlReleases(repoOwner string, repoName string) map[string]string {
	releaseCount := utils.GetNumRelease(repoOwner, repoName)
	releaseTags := utils.GetReleaseTags(repoOwner, repoName, releaseCount)

	releases := make(map[string]string, 0)
	for i := 0; i < len(releaseTags); i++ {
		releaseTag := releaseTags[i]

		content := s.CrawlRelease(repoOwner, repoName, releaseTag)

		releases[releaseTag] = content
	}
	return releases
}
