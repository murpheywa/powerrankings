package prlib

import (
	"fmt"
	"regexp"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	nhlWeekFromURLREText = `/page/powerrankings\-([^/]+)/`
	/*
			Examples:
		      1. (Last week: 1) Washington Capitals, 44-13-7
		      2. (2) Chicago Blackhawks, 42-18-5
		      12. (16) Montreal Canadiens, 37-21-8.
		      31. (N/A) Vegas Golden Knights
			Note that (12) has a trailing period!
			Note that (31) has no record!!

			Steps:
			   1. Capture rank number, skip spaces
			   2. Non-capturing, zero or one text in parenthesis representing prior week rank
			      and skip spaces
			   3. Capture team name as all characters that are NOT a comma.
			   4. Either zero or one occurrence of a comma, space, and the team's current record
			   5. End of string
	*/
	nhlRankREText = `^(\d+)\.\s+(?:\([^\)]+\)\s+){0,1}([^,]+)(?:,\s+[\d\-\.]+){0,1}\s*$`
)

var (
	nhlWeekFromURLRE = regexp.MustCompile(nhlWeekFromURLREText)
	nhlRankRE        = regexp.MustCompile(nhlRankREText)
)

type NHLScraper struct {
	scraper     *ScraperUtil
	currentWeek string
	weekURLMap  map[string]string
	forceUpdate bool
}

func NewNHLScraper(forceUpdate bool, replay bool) (pr *NHLScraper) {
	pr = &NHLScraper{
		scraper:     NewScraperUtil("NHL", replay),
		weekURLMap:  map[string]string{},
		forceUpdate: forceUpdate,
	}
	return
}

func (pr *NHLScraper) Load() (err error) {
	err = pr.scraper.LoadDB()
	return
}

func (pr *NHLScraper) CurrentWeekID() string {
	return pr.currentWeek
}

func (pr *NHLScraper) ScrapeHomePage() (err error) {
	pr.currentWeek = ""
	pr.weekURLMap = map[string]string{}

	// Resilliant URL which will have a resilliant link to the weekly NBARankings
	url := `http://www.espn.com/nhl/`
	var doc *goquery.Document
	if doc, _, err = pr.scraper.NewGoQuery(url); err != nil {
		return errex("get", url, err)
	}

	// use CSS selector found with the browser inspector
	// for each, use ndex and item
	doc.Find(`span[class=link-text]`).EachWithBreak(func(index int, item *goquery.Selection) (goOn bool) {
		title := item.Text()
		if title == "Power Rankings" {
			par := item.Parent()
			if par.Is(`a`) {
				if href, hasHref := par.Attr(`href`); hasHref {
					matches := nhlWeekFromURLRE.FindStringSubmatch(href)
					if len(matches) == 2 {
						href = normalizeESPNURL(href)
						pr.currentWeek = matches[1]
						pr.weekURLMap[pr.currentWeek] = href
						return false
					}
				}
			}
		}
		return true
	})

	return
}

func (pr *NHLScraper) ScrapeRankingPage(week, url string) (err error) {
	var doc *goquery.Document
	if doc, _, err = pr.scraper.NewGoQuery(url); err != nil {
		return errex("get", url, err)
	}

	// TODO: NHL does not have prior rankings links

	// See if I need the rankings for this week
	// always force current week
	if !pr.forceUpdate && week != pr.currentWeek {
		_, found := pr.scraper.season.GetWeekItem(week)
		if found {
			return
		}
	}

	scrapeTime := time.Now()
	rankItems := []*RankItem{}
	// Scrape the rankings for this week
	doc.Find("div[class=article-body]").EachWithBreak(func(index int, item *goquery.Selection) bool {
		// get h2 children of the div
		item.ChildrenFiltered("h2").Each(func(index int, item *goquery.Selection) {
			// Surprisingly, Not all rankings have a team hyperlink!!!!
			text := item.Text()
			anchors := item.ChildrenFiltered(`a`)
			l := anchors.Length()
			if l != 0 {
				// get rid of the anchors
				anchors.RemoveAttr(`href`)
				text = item.Text()
			}
			matches := nhlRankRE.FindStringSubmatch(text)
			if len(matches) == 3 {
				rankItem := &RankItem{Rank: I(matches[1]), Team: matches[2]}
				rankItems = append(rankItems, rankItem)
			} else {
				html, _ := item.Html()
				pr.scraper.L().Printf("unexpected text in (presumably) rank text: '%s'", html)
			}
		})

		// Keep looking for div's if no rank items
		return len(rankItems) == 0
	})

	weekItem := &WeekItem{ScrapeDate: NormalizeDate(scrapeTime), WeekID: week, Rankings: rankItems}
	pr.scraper.season.UpdateWeekItem(weekItem)

	return
}

func (pr *NHLScraper) Scrape() (err error) {
	if err = pr.ScrapeHomePage(); err != nil {
		return errex("scrape nba page", "get current rankings url", err)
	}

	if len(pr.weekURLMap) == 0 {
		return errex("scrape nhl page", "get current rankings url",
			fmt.Errorf("Rankings href not found"))
	}

	// NHL does not have past rankings

	url := pr.weekURLMap[pr.currentWeek]
	if err = pr.ScrapeRankingPage(pr.currentWeek, url); err != nil {
		return errex("week "+pr.currentWeek, url, err)
	}

	err = pr.scraper.UpdateDB()

	return
}

func (pr *NHLScraper) ToCSV(weekID string) (csvBytes []byte, err error) {
	csvBytes, err = pr.scraper.ToCSV(weekID)
	return
}
