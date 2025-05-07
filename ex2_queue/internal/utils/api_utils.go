package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
)

var baseURL = "https://github.com"

func GetRepoURL(repo string) string {
	return baseURL + "repos/" + repo
}

func GetNumRelease(repoOwner string, repoName string) int {
	repoURL := baseURL + "/" + repoOwner + "/" + repoName

	c := colly.NewCollector()

	numRelease := 0

	c.OnRequest(func(r *colly.Request) {
		// fmt.Println("Visiting", r.URL)
	})

	c.OnHTML("a.Link--primary.no-underline.Link", func(e *colly.HTMLElement) {
		text := e.Text
		if strings.Contains(text, "Releases") {
			// fmt.Println("Text:", text)
			re := regexp.MustCompile(`\d+`)
			match := re.FindString(text)
			numRelease, _ = strconv.Atoi(match)
			// fmt.Println("Number of releases:", numRelease)
		}
	})

	err := c.Visit(repoURL)
	if err != nil {
		fmt.Println("Error visiting URL:", err)
	}

	return numRelease
}

func GetReleaseTags(owner string, repo string, numRelease int) []string {
	log := logrus.New()
	releaseURL := baseURL + "/" + owner + "/" + repo + "/releases"

	c := colly.NewCollector()

	c.OnRequest(func(r *colly.Request) {
	})
	tags := make([]string, 0, numRelease)

	c.OnHTML("a.Link--primary.Link", func(e *colly.HTMLElement) {
		tagHref := strings.Split(e.Attr("href"), "/")
		tag := tagHref[len(tagHref)-1]
		tags = append(tags, tag)
		// fmt.Println(tag)
	})

	currentPage := 1
	for true {
		if len(tags) >= numRelease {
			break
		}
		visitURL := releaseURL + "?page=" + strconv.Itoa(currentPage)
		if err := c.Visit(visitURL); err != nil {
			log.WithError(err).Errorf("Error visiting %s: %v", visitURL, err)
			break

		}
		currentPage++
	}

	return tags
}

func GetReleaseURLs(repo string, tags []string) []string {
	releaseURLs := make([]string, len(tags))
	for i, tag := range tags {
		releaseURLs[i] = baseURL + repo + "/releases/tag/" + tag
	}
	return releaseURLs
}

func GetCommitURLs(repo string, sha string) string {
	return baseURL + "repos/" + repo + "/commits/" + sha
}

func GetNumCommitRelease(releaseURL string) int {
	log := logrus.New()
	c := colly.NewCollector()

	c.OnRequest(func(r *colly.Request) {
		log.Debug("Visiting release URL: ", r.URL)
	})

	numCommits := 0

	// Try multiple selectors since GitHub's HTML structure may vary
	c.OnHTML("div.d-flex.flex-row.flex-wrap.color-fg-muted.flex-items-end", func(e *colly.HTMLElement) {
		text := e.Text
		log.Debug("Found commit info text: ", text)

		// Look for patterns like "123 commits" or "1,234 commits"
		re := regexp.MustCompile(`([\d,]+)\s+commits?`)
		match := re.FindStringSubmatch(text)

		// Only proceed if we found a match
		if len(match) > 1 {
			// Remove commas from numbers like "1,234"
			commitStr := strings.ReplaceAll(match[1], ",", "")
			count, err := strconv.Atoi(commitStr)
			if err == nil {
				numCommits = count
				log.Infof("Found %d commits in release", numCommits)
			}
		}
	})

	// Alternative selector for commit count
	c.OnHTML("span.d-none.d-sm-inline", func(e *colly.HTMLElement) {
		text := e.Text
		if strings.Contains(text, "commit") {
			log.Debug("Found alternate commit info: ", text)

			re := regexp.MustCompile(`([\d,]+)\s+commits?`)
			match := re.FindStringSubmatch(text)

			if len(match) > 1 {
				commitStr := strings.ReplaceAll(match[1], ",", "")
				count, err := strconv.Atoi(commitStr)
				if err == nil && count > numCommits {
					numCommits = count
					log.Infof("Found %d commits using alternative selector", numCommits)
				}
			}
		}
	})

	// Try one more selector for commit comparison page
	c.OnHTML("div.Box-header span.text-emphasized", func(e *colly.HTMLElement) {
		text := e.Text
		if strings.Contains(text, "Showing") || strings.Contains(text, "commit") {
			log.Debug("Found header commit info: ", text)

			re := regexp.MustCompile(`([\d,]+)`)
			match := re.FindString(text)

			if match != "" {
				commitStr := strings.ReplaceAll(match, ",", "")
				count, err := strconv.Atoi(commitStr)
				if err == nil && count > numCommits {
					numCommits = count
					log.Infof("Found %d commits using Box-header selector", numCommits)
				}
			}
		}
	})

	// For debugging, let's look at the whole page structure if we don't find commits
	c.OnScraped(func(r *colly.Response) {
		if numCommits == 0 {
			log.Warn("No commits found on release page, URL: ", releaseURL)
		}
	})

	if err := c.Visit(releaseURL); err != nil {
		log.WithError(err).Errorf("Error visiting %s", releaseURL)
	}

	return numCommits
}

// func main() {
// 	repo := "/opencv/opencv"
// 	fmt.Print(GetTags(repo))
// }
