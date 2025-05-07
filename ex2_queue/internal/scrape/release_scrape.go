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
	// Clone the collector to avoid sharing state between requests
	c := s.Colly.Clone()

	releaseURL := "https://github.com/" + repoOwner + "/" + repoName + "/releases/tag/" + releaseTag
	s.Log.WithFields(logrus.Fields{
		"owner": repoOwner,
		"repo":  repoName,
		"tag":   releaseTag,
		"url":   releaseURL,
	}).Info("Scraping release")

	contentData := ""

	c.OnHTML("div.Box-body", func(e *colly.HTMLElement) {
		e.DOM.Find("div.markdown-body.my-3").Each(func(i int, s *goquery.Selection) {
			html, _ := s.Html()
			contentData += html + "\n" // Store HTML instead of plain text
		})
	})

	err := c.Visit(releaseURL)
	if err != nil {
		s.Log.WithError(err).Error("Error visiting release URL")
		return ""
	}

	// Wait for all requests to finish
	c.Wait()

	s.Log.WithFields(logrus.Fields{
		"tag":            releaseTag,
		"content_length": len(contentData),
	}).Info("Content scraped")

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
