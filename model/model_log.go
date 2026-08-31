package model

import (
	"html"
	"html/template"
	"log"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/util"
	"github.com/dlclark/regexp2/v2"
	"github.com/dlclark/regexp2/v2/compat"
)

type Log struct {
	ID        int
	Account   *Account
	Board     *Board
	Timestamp int64
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

	rgxp, err := compat.Compile(`&gt;&gt;/([0-9A-Za-z_-]+)/([0-9]+)`)
	if err != nil {
		log.Fatal(err)
	}
	message = ReplaceAllStringFunc(rgxp, message, func(match regexp2.Match) string {
		s := match.String()
		if strings.HasPrefix(s, "&gt;&gt;/post/") {
			return ReplaceAllString(rgxp, s, `<a href="/sriracha/$1/$2">&gt;&gt;$2</a>`)
		}
		return ReplaceAllString(rgxp, s, `<a href="/sriracha/$1/$2">$1 #$2</a>`)
	})

	rgxp, err = compat.Compile(`&gt;&gt;/([0-9A-Za-z_-]+)/([0-9A-Za-z_-]+)`)
	if err != nil {
		log.Fatal(err)
	}
	return template.HTML(ReplaceAllStringFunc(rgxp, message, func(match regexp2.Match) string {
		return ReplaceAllString(rgxp, match.String(), `<a href="/sriracha/$1/$2">$1 $2</a>`)
	}))
}

func (l *Log) MessageLabel() template.HTML {
	return l.formatLabel(l.Message)
}

func (l *Log) InfoLabel() template.HTML {
	return l.formatLabel(l.Changes)
}
