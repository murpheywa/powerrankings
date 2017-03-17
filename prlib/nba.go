package prlib

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	nbaWeekFromURLREText = `week\-(\d+)\-rankings`
	nbaWeekIDREText      = `^(?:Week ){0,1}(\d+|Camp)`
	nbaRankREText        = `^(\d+)\.\s+(.*)`
)

var (
	nbaWeekFromURLRE = regexp.MustCompile(nbaWeekFromURLREText)
	nbaWeekIDRE      = regexp.MustCompile(nbaWeekIDREText)
	nbaRankRE        = regexp.MustCompile(nbaRankREText)
)

func errex(step, context string, err error) error {
	if err == nil {
		return err
	}

	return fmt.Errorf("%s: %s: %v", step, context, err)
}

type NBAScraper struct {
	scraper     *ScraperUtil
	currentWeek string
	weekURLMap  map[string]string
	forceUpdate bool
}

func normalizeESPNURL(url string) string {
	if strings.Index(strings.ToLower(url), "http://") == 0 {
		return url
	}
	return "http://www.espn.com" + url
}

func NewNBAScraper(forceUpdate bool, replay bool) (pr *NBAScraper) {
	pr = &NBAScraper{
		scraper:     NewScraperUtil("NBA", replay),
		weekURLMap:  map[string]string{},
		forceUpdate: forceUpdate,
	}
	return
}

func (pr *NBAScraper) Load() (err error) {
	err = pr.scraper.LoadDB()
	return
}

func (pr *NBAScraper) CurrentWeekID() string {
	return pr.currentWeek
}

func (pr *NBAScraper) ScrapeHomePage() (err error) {
	pr.currentWeek = ""
	pr.weekURLMap = map[string]string{}

	// Resilliant URL which will have a resilliant link to the weekly NBARankings
	url := `http://www.espn.com/nba/`
	var doc *goquery.Document
	if doc, _, err = pr.scraper.NewGoQuery(url); err != nil {
		return errex("get", url, err)
	}

	// use CSS selector found with the browser inspector
	// for each, use ndex and item
	doc.Find(`span[class=link-text]`).EachWithBreak(func(index int, item *goquery.Selection) (goOn bool) {
		title := item.Text()
		if title == "Rankings" {
			par := item.Parent()
			if par.Is(`a`) {
				if href, hasHref := par.Attr(`href`); hasHref {
					matches := nbaWeekFromURLRE.FindStringSubmatch(href)
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

func (pr *NBAScraper) scrapeWeekTOC(doc *goquery.Document) (err error) {
	doc.Find(`p em strong`).EachWithBreak(func(index int, item *goquery.Selection) (goOn bool) {
		text := item.Text()
		if text == "Previous rankings:" {
			// Go back to the `p` parent, then get all `a` children
			item.ParentsFiltered(`p`).ChildrenFiltered(`a`).Each(func(ii int, anchorItem *goquery.Selection) {
				text := anchorItem.Text()
				matches := nbaWeekIDRE.FindStringSubmatch(text)
				if len(matches) != 2 {
					return
				}
				week := matches[1]
				href := anchorItem.AttrOr(`href`, "")
				href = normalizeESPNURL(href)
				pr.weekURLMap[week] = href
			})

			return false
		}

		return true
	})

	return
}

func (pr *NBAScraper) ScrapeRankingPage(week, url string) (err error) {
	var doc *goquery.Document
	var html []byte
	if doc, html, err = pr.scraper.NewGoQuery(url); err != nil {
		return errex("get", url, err)
	}

	_ = html

	if len(pr.weekURLMap) == 1 {
		if err = pr.scrapeWeekTOC(doc); err != nil {
			return errex("ScrapeRankingPage", "week TOC", err)
		}
	}

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
	doc.Find("#article-feed article .container b").Each(func(index int, item *goquery.Selection) {
		// should have an anchor children
		anchors := item.ChildrenFiltered(`a`)
		l := anchors.Length()
		if l != 0 {
			// get rid of the anchors
			anchors.RemoveAttr(`href`)
			text := item.Text()
			matches := nbaRankRE.FindStringSubmatch(text)
			if len(matches) == 3 {
				rankItem := &RankItem{Rank: I(matches[1]), Team: matches[2]}
				rankItems = append(rankItems, rankItem)
			} else {
				pr.scraper.L().Printf("unexpected text in (presumably) rank text: '%s'", text)
			}
		}
	})

	weekItem := &WeekItem{ScrapeDate: NormalizeDate(scrapeTime), WeekID: week, Rankings: rankItems}
	pr.scraper.season.UpdateWeekItem(weekItem)

	return
}

func (pr *NBAScraper) Scrape() (err error) {
	if err = pr.ScrapeHomePage(); err != nil {
		return errex("scrape nba page", "get current rankings url", err)
	}

	if len(pr.weekURLMap) == 0 {
		return errex("scrape nba page", "get current rankings url",
			fmt.Errorf("Rankings href not found"))
	}

	url := pr.weekURLMap[pr.currentWeek]
	if err = pr.ScrapeRankingPage(pr.currentWeek, url); err != nil {
		return errex("week "+pr.currentWeek, url, err)
	}

	for k, v := range pr.weekURLMap {
		if k == pr.currentWeek {
			continue
		}

		if err = pr.ScrapeRankingPage(k, v); err != nil {
			return errex("week "+k, v, err)
		}
	}

	err = pr.scraper.UpdateDB()

	return
}

func (pr *NBAScraper) ToCSV(weekID string) (csvBytes []byte, err error) {
	csvBytes, err = pr.scraper.ToCSV(weekID)
	return
}
