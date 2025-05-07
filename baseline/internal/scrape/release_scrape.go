package scrape

import (
	"crawler/baseline/internal/utils"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
)

func CrawlReleases(repoOwner string, repoName string) map[string]string {
	releaseCount := utils.GetNumRelease(repoOwner, repoName)
	releaseTags := utils.GetReleaseTags(repoOwner, repoName, releaseCount)

	releases := make(map[string]string, 0)
	for i := 0; i < len(releaseTags); i++ {
		releaseTag := releaseTags[i]

		content := CrawlRelease(repoOwner, repoName, releaseTag)

		releases[releaseTag] = content
	}
	return releases
}

func CrawlRelease(repoOwner string, repoName string, releaseTag string) string {
	log := logrus.New()
	releaseURL := "https://github.com/" + repoOwner + "/" + repoName + "/releases/tag/" + releaseTag
	c := colly.NewCollector()
	// log.Info("Starting to scrape release: ", releaseURL)
	c.OnRequest(func(req *colly.Request) {
		// log.Info("visiting: ", releaseURL)
	})
	contentData := ""
	c.OnHTML("div.Box-body", func(e *colly.HTMLElement) {
		e.DOM.Find("div.markdown-body.my-3").Each(func(i int, s *goquery.Selection) {
			contentData += s.Text() + "\n"
		})
	})

	err := c.Visit(releaseURL)
	if err != nil {
		log.Error("Error visiting release URL: ", err)
		return ""
	}
	log.Info("Scraping completed for release: ", releaseTag)
	// log.Info("Content: ", contentData)
	return contentData
}
