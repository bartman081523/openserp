package google

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/karust/openserp/core"
	"github.com/sirupsen/logrus"
)

type Google struct {
	core.Browser
	findNumRgxp  *regexp.Regexp
	checkTimeout time.Duration
}

func New(browser core.Browser) *Google {
	gogl := Google{Browser: browser}
	gogl.checkTimeout = time.Second * 5
	gogl.findNumRgxp = regexp.MustCompile("\\d")
	return &gogl
}
func (gogl *Google) Name() string {
	return "google"
}

func (gogl *Google) FindTotalResults(page *rod.Page) (int, error) {
	resultsStats, err := page.Timeout(gogl.checkTimeout).Search("div#result-stats")
	if err != nil {
		return 0, errors.New("Result stats not found: " + err.Error())
	}
	stats, err := resultsStats.First.Text()
	if err != nil {
		return 0, errors.New("Cannot extract result stats text: " + err.Error())
	}

	// Escape moment with `seconds` and extract digits
	allNums := gogl.findNumRgxp.FindAllString(stats[:len(stats)-15], -1)
	stats = strings.Join(allNums, "")

	total, err := strconv.Atoi(stats)
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (gogl *Google) preparePage(page *rod.Page) {
	// Remove "similar queries" lists
	page.Eval(";(() => { document.querySelectorAll(`div[data-initq]`).forEach( el => el.remove());  })();")
}

func (gogl *Google) Search(query core.Query) ([]core.SearchResult, error) {
	logrus.Tracef("Start Google search, query: %+v", query)

	searchResults := []core.SearchResult{}

	// Build URL from query struct to open in browser
	url, err := BuildURL(query)
	if err != nil {
		return nil, err
	}

	page := gogl.Navigate(url)
	gogl.preparePage(page)

	totalResults, err := gogl.FindTotalResults(page)
	if err != nil {
		return nil, err
	}
	logrus.Tracef("%d total results found", totalResults)

	if totalResults == 0 {
		return searchResults, nil
	}

	results, err := page.Timeout(gogl.Timeout).Search("div[data-hveid][data-ved][lang]")
	if err != nil {
		return nil, err
	}

	resultElements, err := results.All()
	if err != nil {
		return nil, err
	}

	for i, r := range resultElements {
		// Get URL
		link, err := r.Element("a")
		if err != nil {
			continue
		}
		linkText, err := link.Property("href")
		if err != nil {
			logrus.Error("No `href` tag found")
		}

		// Get title
		titleTag, err := link.Element("h3")
		if err != nil {
			logrus.Error("No `h3` tag found")
			continue
		}

		title, err := titleTag.Text()
		if err != nil {
			logrus.Error("Cannot extract text from title")
			title = "No title"
		}

		// Get description
		// doesn't catch all
		descTag, err := r.Element(`div[data-sncf~="1"]`)
		desc := ""
		if err != nil {
			logrus.Trace(`No description 'div[data-sncf~="1"]' tag found`)
		} else {
			desc = descTag.MustText()
		}

		gR := core.SearchResult{Rank: i + 1, URL: linkText.String(), Title: title, Description: desc}
		searchResults = append(searchResults, gR)
	}

	if !gogl.Browser.LeavePageOpen {
		err = page.Close()
		if err != nil {
			logrus.Error(err)
		}
	}

	return searchResults, nil
}
