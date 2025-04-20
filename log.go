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
	Changes   string
}

func (l *Log) TimestampDate() string {
	return time.Unix(l.Timestamp, 0).Format("2006-01-02 15:04:05 MST")
}

func (l *Log) formatLabel(message string) template.HTML {
	if len(message) == 0 {
		return ""
	}
	message = html.EscapeString(message)
	rgxp, err := regexp.Compile(`&gt;&gt;/([0-9A-Za-z_-]+)/([0-9]+)`)
	if err != nil {
		log.Fatal(err)
	}
	return template.HTML(rgxp.ReplaceAllString(message, `<a href="/sriracha/$1/$2">$1 #$2</a>`))
}

func (l *Log) MessageLabel() template.HTML {
	return l.formatLabel(l.Message)
}

func (l *Log) InfoLabel() template.HTML {
	return l.formatLabel(l.Changes)
}
