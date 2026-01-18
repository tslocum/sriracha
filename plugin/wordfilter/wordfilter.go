package main

import (
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
)

const (
	configWordfilters     = "wordfilters"
	configWordfiltersInfo = `Line 1 of each wordfilter is the regular expression to search for.
Line 2 is the replacement text. Capture groups may be referenced as $1, $2, etc.
Line 3 is optional, and may be set to a list of board IDs separated by commas.
When line 3 is set, the wordfilter only applies to the specified boards.
Board IDs are available in the update board URL.`
)

type filter struct {
	Search  string
	Replace string
	Boards  []int

	pattern *regexp.Regexp
}

type Wordfilter struct {
	filters []*filter
}

func (w *Wordfilter) About() string {
	return "Find and replace text in post messages."
}

func (w *Wordfilter) Config() []sriracha.PluginConfig {
	return []sriracha.PluginConfig{
		{
			Type:     sriracha.TypeString,
			Multiple: true,
			Name:     configWordfilters,
			Default:  "wordfilter\ncheesegrater",
			Info:     configWordfiltersInfo,
		},
	}
}

func (w *Wordfilter) Update(db sriracha.DB, key string) error {
	if key == configWordfilters {
		var filters []*filter
		for _, v := range db.GetMultiString(configWordfilters) {
			split := strings.Split(v, "\n")
			if len(split) < 2 {
				continue
			}

			pattern, err := regexp.Compile(split[0])
			if err != nil {
				log.Printf("warning: failed to parse `%s` as regular expression: %s", split[0], err)
				continue
			}

			f := &filter{
				Search:  split[0],
				Replace: split[1],
				pattern: pattern,
			}
			if len(split) > 2 {
				for _, vv := range strings.Split(split[2], ",") {
					boardID, err := strconv.Atoi(vv)
					if err == nil && boardID > 0 {
						f.Boards = append(f.Boards, boardID)
					}
				}
			}
			filters = append(filters, f)
		}
		w.filters = filters
	}
	return nil
}

func (w *Wordfilter) Post(db sriracha.DB, post *Post) error {
	for _, f := range w.filters {
		if len(f.Boards) != 0 && !slices.Contains(f.Boards, post.Board.ID) {
			continue
		}
		post.Message = f.pattern.ReplaceAllString(post.Message, f.Replace)
	}
	return nil
}

func Plugin() any {
	return &Wordfilter{}
}

func main() {}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin           = &Wordfilter{}
	_ sriracha.PluginWithConfig = &Wordfilter{}
	_ sriracha.PluginWithUpdate = &Wordfilter{}
	_ sriracha.PluginWithPost   = &Wordfilter{}
)
