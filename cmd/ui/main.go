package main

import (
	"flag"
	"time"
)
import "github.com/rivo/tview"

var (
	addr     = flag.String("addr", "http://localhost:8080", "api server address")
	timezone = flag.String("timezone", "Asia/Shanghai", "timezone")
)

func main() {
	flag.Parse()
	app := tview.NewApplication()

	clockInfo := tview.NewTextView()
	clockInfo.SetBorder(true).SetTitle("Current Time")
	clockInfo.SetTextAlign(tview.AlignCenter)

	go updateClock(clockInfo)

	topArea := tview.NewFlex().
		AddItem(clockInfo, 0, 1, false)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topArea, 0, 1, false)

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func updateClock(clockInfo *tview.TextView) {
	for {
		location, err := time.LoadLocation(*timezone)
		if err != nil {
			panic(err)
		}
		clockInfo.SetText(time.Now().In(location).Format("2006-01-02 15:04:05"))
		time.Sleep(time.Second)
	}
}
