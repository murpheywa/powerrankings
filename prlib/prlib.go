package prlib

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"encoding/csv"
	"encoding/json"
	"os"
	"path"
	"strconv"

	"github.com/PuerkitoBio/goquery"
)

const (
	defaultUA = `Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36`
)

var ProjectDirectory string

func init() {
	// NOTE: This code can take a half minute or so
	// if the user is a domain user and the computer ios NOT
	// on the domain (sigh)
	// ----------------------------------------
	// usr, err := user.Current()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// ProjectDirectory = usr.HomeDir
	// ----------------------------------------
	if runtime.GOOS == "windows" {
		ProjectDirectory = os.Getenv("LOCALAPPDATA")
	} else {
		home := os.Getenv("HOME")
		ProjectDirectory = path.Join(home, ".config", "murpheywa", "powerrankings")
	}
	os.MkdirAll(ProjectDirectory, 0644)
}

func S(val int) (s string) {
	s = strconv.Itoa(val)
	return
}

func I(text string) int {
	val, _ := strconv.Atoi(text)
	return val
}

func mkdirParent(filePath string) {
	dir := path.Dir(filePath)
	os.MkdirAll(dir, 0644)
	return
}

type ScraperUtil struct {
	league    string
	dbPath    string
	season    *SeasonDB
	cache     *HttpQueryCache
	logBuffer bytes.Buffer
	logger    *log.Logger
}

func NewScraperUtil(league string, replay bool) (ps *ScraperUtil) {
	cachePath := path.Join(ProjectDirectory, "httpcache", league+".txt")
	mkdirParent(cachePath)
	dbPath := path.Join(ProjectDirectory, "db", league+".json")
	mkdirParent(dbPath)

	ps = &ScraperUtil{
		league:    league,
		dbPath:    dbPath,
		cache:     NewHttpQueryCache(cachePath, replay),
		season:    NewSeasonDB(league),
		logBuffer: bytes.Buffer{},
	}
	ps.logger = log.New(&ps.logBuffer, "logger: ", log.Lshortfile)

	return
}

func (ss *ScraperUtil) L() *log.Logger {
	return ss.logger
}

func (ss *ScraperUtil) LoadDB() (err error) {
	var b []byte
	if b, err = ioutil.ReadFile(ss.dbPath); err != nil {
		if !os.IsNotExist(err) {
			return
		}
		b, _ = json.Marshal(ss.season)
		if err = ioutil.WriteFile(ss.dbPath, b, 0644); err != nil {
			return
		}
	}

	season := &SeasonDB{}
	if err = json.Unmarshal(b, season); err != nil {
		return
	}
	ss.season = season

	return
}

func (ss *ScraperUtil) UpdateDB() (err error) {
	var b []byte
	if b, err = json.Marshal(ss.season); err != nil {
		return errex("UpdateDB", "json.Marshal", err)
	}
	if err = ioutil.WriteFile(ss.dbPath, b, 0644); err != nil {
		return errex("UpdateDB", "write to "+ss.dbPath, err)
	}

	return
}

func (ss *ScraperUtil) ToCSV(weekID string) (csvBytes []byte, err error) {
	weekItem, found := ss.season.GetWeekItem(weekID)
	if !found {
		err = fmt.Errorf("week not found: '%s'", weekID)
		return
	}

	header := []string{
		"league",
		"week",
		"team",
		"rank",
	}
	outb := bytes.Buffer{}
	writer := csv.NewWriter(&outb)
	if err = writer.Write(header); err != nil {
		return
	}
	for _, v := range weekItem.Rankings {
		rec := []string{
			ss.league,
			weekID,
			v.Team,
			S(v.Rank),
		}
		if err = writer.Write(rec); err != nil {
			return
		}
	}

	writer.Flush()
	csvBytes = outb.Bytes()

	return
}

func (ss *ScraperUtil) NewGoQuery(url string) (doc *goquery.Document, html []byte, err error) {
	if html, err = ss.cache.Get(url, defaultUA); err != nil {
		return
	}
	reader := bytes.NewReader(html)
	doc, err = goquery.NewDocumentFromReader(reader)
	return
}

type HttpQueryCache struct {
	CachePath string
	history   []string
}

func NewHttpQueryCache(cachePath string, replay bool) (c *HttpQueryCache) {
	c = &HttpQueryCache{
		CachePath: cachePath,
	}
	var err error
	b := []byte{}
	if replay {
		b = []byte{}
		if b, err = ioutil.ReadFile(c.CachePath); err != nil {
			return
		}
		if len(b) > 0 {
			c.history = strings.Split(string(b), "\n")
			// last element will be blank
			lastIdx := len(c.history) - 1
			if lastIdx >= 0 && c.history[lastIdx] == "" {
				c.history = c.history[0:lastIdx]
			}
		}
	} else {
		c.history = nil
		ioutil.WriteFile(c.CachePath, []byte{}, 0644)
	}

	return
}

func (qc *HttpQueryCache) Get(url, ua string) (html []byte, err error) {
	if len(qc.history) == 0 {
		client := &http.Client{}
		var req *http.Request
		req, err = http.NewRequest("GET", url, nil)
		req.Header.Add("User-Agent", ua)
		var resp *http.Response
		if resp, err = client.Do(req); err != nil {
			return
		}
		defer resp.Body.Close()
		if html, err = ioutil.ReadAll(resp.Body); err != nil {
			return
		}

		js, _ := json.Marshal(&html)
		// append
		fp, _ := os.OpenFile(qc.CachePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		defer fp.Close()
		fp.Write(js)
		fp.WriteString("\n")
	} else {
		js := qc.history[0]
		qc.history = qc.history[1:]
		json.Unmarshal([]byte(js), &html)
	}

	debugPath := path.Join(path.Dir(qc.CachePath), "current.htm")
	ioutil.WriteFile(debugPath, html, 0644)

	return
}

func NormalizeDate(dt time.Time) (text string) {
	text = dt.Format("2006-01-02 15:04:05")
	return
}
