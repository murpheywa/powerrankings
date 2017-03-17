package prlib

import "sort"
import "strconv"

type SeasonDB struct {
	League    string      `json:"league"`
	WeekItems []*WeekItem `json:"weeks"`
}

func (sdb *SeasonDB) internalGetWeek(id string) (index int, item *WeekItem) {
	for index, item = range sdb.WeekItems {
		if item.WeekID == id {
			return
		}
	}
	index = -1
	item = nil
	return
}

func (sdb *SeasonDB) GetWeekItem(id string) (item *WeekItem, found bool) {
	var index int
	index, item = sdb.internalGetWeek(id)
	found = (index >= 0)
	return
}

func (sdb *SeasonDB) UpdateWeekItem(item *WeekItem) {
	item.SortRankings()
	if idx, _ := sdb.internalGetWeek(item.WeekID); idx >= 0 {
		sdb.WeekItems[idx] = item
	} else {
		sdb.WeekItems = append(sdb.WeekItems, item)
	}
	sdb.SortWeekItems()
}

func (sdb *SeasonDB) SortWeekItems() {
	items := sdb.WeekItems
	sort.Slice(items, func(l, r int) bool {
		wl := items[l].WeekID
		wr := items[r].WeekID
		il, ilErr := strconv.ParseInt(wl, 10, 63)
		ir, irErr := strconv.ParseInt(wr, 10, 63)

		if ilErr == nil {
			// Here, left is a number
			if irErr == nil { // both are numbers
				return (il < ir)
			}
			// numbers are always more than strings
			return false
		}

		// Here, left is not a number
		if irErr != nil { // both not numbers
			return (wl < wr)
		}
		// strings are always less than numbers
		return true
	})
}

type WeekItem struct {
	WeekID     string      `json:"week"`
	ScrapeDate string      `json:"modified"`
	Rankings   []*RankItem `json:"rankings"`
}

func (wi *WeekItem) SortRankings() {
	// sort in place by rank
	rankItems := wi.Rankings
	sort.Slice(rankItems, func(l, r int) bool {
		return (rankItems[l].Rank < rankItems[r].Rank)
	})
}

type RankItem struct {
	Team string `json:"team"`
	Rank int    `json:"rank"`
}

func NewSeasonDB(league string) (db *SeasonDB) {
	db = &SeasonDB{
		League:    league,
		WeekItems: []*WeekItem{},
	}
	return
}

type PRScraper interface {
	Load() error
	Scrape() error
	CurrentWeekID() string
	ToCSV(weekID string) ([]byte, error)
}
