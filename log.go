package sriracha

import (
	"html"
	"html/template"
	"log"
	"regexp"
	"time"
)

type Log struct {
	ID        int
	Board     *Board
	Timestamp int64
	Account   *Account
	Message   string
}

func (l *Log) TimestampDate() string {
	return time.Unix(l.Timestamp, 0).Format("2006-01-02 15:04:05 MST")
}

func (l *Log) MessageLabel() template.HTML {
	if len(l.Message) == 0 {
		return ""
	}
	message := html.EscapeString(l.Message)
	rgxp, err := regexp.Compile(`&gt;&gt;/([0-9A-Za-z_-]+)/([0-9]+)`)
	if err != nil {
		log.Fatal(err)
	}
	return template.HTML(rgxp.ReplaceAllString(message, `<a href="/imgboard/$1/$2">$1 #$2</a>`))
}
