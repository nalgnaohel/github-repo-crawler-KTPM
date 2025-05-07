package scrape

import (
	"crawler/baseline/internal/utils"
	"fmt"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
)

func CrawlCommit(repoOwner string, repoName string, releaseTag string) []string {
	log := logrus.New()

	// Try master branch first
	commits := tryBranch(repoOwner, repoName, releaseTag, "master", log)

	// If no commits found with master, try main branch
	if len(commits) == 0 {
		log.Info("No commits found with master branch, trying main branch")
		commits = tryBranch(repoOwner, repoName, releaseTag, "main", log)
	}

	log.Infof("Total unique commits found: %d", len(commits))
	return commits
}

// tryBranch attempts to crawl commits using a specific branch name
func tryBranch(repoOwner string, repoName string, releaseTag string, branchName string, log *logrus.Logger) []string {
	releaseURL := "https://github.com/" + repoOwner + "/" + repoName + "/releases/tag/" + releaseTag
	commitCount := utils.GetNumCommitRelease(releaseURL)

	baseURL := fmt.Sprintf("https://github.com/%s/%s/compare/commit-list?range=%s...%s",
		repoOwner, repoName, releaseTag, branchName)

	log.Infof("Trying to crawl commits with branch: %s", branchName)

	c := colly.NewCollector()

	c.OnResponse(func(r *colly.Response) {
		log.Info("Received response with status: ", r.StatusCode)
	})

	c.OnRequest(func(req *colly.Request) {
		// log.Info("Visiting: ", req.URL.String())
	})

	// Use a map to efficiently track commits by hash and combine messages
	commitMap := make(map[string]string)

	// Look for the commit list items
	c.OnHTML("div.TimelineItem-body", func(e *colly.HTMLElement) {
		// Extract commit information from the Link--primary element which has the commit link and message
		commitHash := ""
		commitMsg := ""

		// Find the commit link which contains both the hash and message
		e.ForEach("p.mb-1 a.Link--primary", func(_ int, link *colly.HTMLElement) {
			href := link.Attr("href")
			if strings.Contains(href, "/commit/") {
				parts := strings.Split(href, "/commit/")
				if len(parts) > 1 {
					commitHash = parts[1]
					// Get the commit message from the link text
					commitMsg = strings.TrimSpace(link.Text)

					if commitHash != "" {
						// If we already have this hash, append the new message
						if existingMsg, ok := commitMap[commitHash]; ok {
							commitMap[commitHash] = existingMsg + " | " + commitMsg
							log.Infof("Updated commit %s with additional message: %s", commitHash, commitMsg)
						} else {
							// New hash, create a new entry
							commitMap[commitHash] = commitMsg
							log.Infof("Found new commit: %s - %s", commitHash, commitMsg)
						}
					}
				}
			}
		})
	})

	// Look for "no commits" message
	hasCommits := true
	c.OnHTML("div.blankslate", func(e *colly.HTMLElement) {
		if strings.Contains(e.Text, "There aren't any commits") {
			hasCommits = false
			log.Infof("No commits found with branch: %s", branchName)
		}
	})

	// Print commit count from the page header
	c.OnHTML("div.Box-header", func(e *colly.HTMLElement) {
		countText := e.ChildText("span.text-emphasized")
		log.Info("Commit count info: ", countText)
	})

	// Debug selectors to check structure
	c.OnHTML("div.js-navigation-container", func(e *colly.HTMLElement) {
		log.Info("Found commit container with child count: ", len(e.DOM.Children().Nodes))
	})

	// For pagination
	page := 1
	maxPages := (commitCount + 49) / 50 // Each page has ~50 commits

	// Visit the first page
	err := c.Visit(baseURL)
	if err != nil {
		log.Errorf("Error visiting URL with branch %s: %v", branchName, err)
		return []string{}
	}

	// If no commits found on first page, return early
	if !hasCommits {
		return []string{}
	}

	// Continue with pagination if needed
	for page < maxPages {
		page++
		commitURL := fmt.Sprintf("%s&page=%d", baseURL, page)

		// log.Infof("Visiting page %d of %d", page, maxPages)
		err := c.Visit(commitURL)
		if err != nil {
			log.Error("Error visiting commit URL: ", err)
			break
		}

		log.Infof("Completed page %d", page)
	}

	// Convert the map to a slice of formatted strings
	commits := make([]string, 0, len(commitMap))
	for hash, message := range commitMap {
		commitInfo := fmt.Sprintf("Hash: %s - Message: %s", hash, message)
		commits = append(commits, commitInfo)
	}

	log.Infof("Found %d commits with branch: %s", len(commits), branchName)
	return commits
}
