package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/murpheywa/powerrankingsgo/prlib"
)

var (
	leagueName  string
	weekID      string
	forceUpdate bool
	replay      bool
)

func errExit(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(2)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "%v\n", r)
			os.Exit(2)
		}
	}()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: powerrankings -l leagueName [-w weekID] [outFile]\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&leagueName, "l", "", "League Name, no default")
	flag.BoolVar(&forceUpdate, "f", false, "Force update of existing weeks")
	flag.BoolVar(&replay, "r", false, "Re-run last scrape")
	flag.StringVar(&weekID, "w", "", "Week ID, Default is current week")
	flag.Parse()
	if leagueName == "" || flag.NArg() > 1 {
		flag.Usage()
		os.Exit(1)
	}

	var err error
	var scraper prlib.PRScraper
	switch strings.ToLower(leagueName) {
	case "nba":
		scraper = prlib.NewNBAScraper(forceUpdate, replay)
	case "nhl":
		scraper = prlib.NewNHLScraper(forceUpdate, replay)
	default:
		panic(fmt.Errorf("Unknown league: '%s'", leagueName))
	}

	if err = scraper.Load(); err != nil {
		panic(err)
	}
	if err := scraper.Scrape(); err != nil {
		panic(err)
	}
	// todo: non-fatal errors from scrape

	if weekID == "" {
		weekID = scraper.CurrentWeekID()
	}
	var b []byte
	if b, err = scraper.ToCSV(weekID); err != nil {
		panic(err)
	}

	if flag.NArg() == 1 {
		outName := flag.Arg(0)
		if err = ioutil.WriteFile(outName, b, 0644); err != nil {
			panic(err)
		}
	} else {
		fmt.Printf("%s", string(b))
	}
}
