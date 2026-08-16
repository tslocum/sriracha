package server

import (
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
)

func (s *Server) servePerformance(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}

	type timing struct {
		path string
		ms   int32
	}

	data.Template = "manage_performance"
	s.pageTimingLock.Lock()
	timings := make([]timing, len(s.pageTimings))
	var i int
	for path, ms := range s.pageTimings {
		if ms == 0 {
			ms = 1
		}
		timings[i].path, timings[i].ms = path, ms
		i++
	}
	s.pageTimingLock.Unlock()

	var sortName bool
	slices.SortFunc(timings, func(a timing, b timing) int {
		if sortName {
			return strings.Compare(a.path, b.path)
		}
		delta := b.ms - a.ms
		switch {
		case delta < 0:
			return -1
		case delta > 0:
			return 1
		default: // delta == 0
			return strings.Compare(a.path, b.path)
		}
	})
	data.Message = template.HTML(fmt.Sprintf(`<table class="managetable"><tbody><tr><th>Millis</th><th>%s</th></tr>`, data.G("Page")))
	for _, t := range timings {
		data.Message += template.HTML(fmt.Sprintf(`<tr><td>%d</td><td><a href="%s">%s</a></td></tr>`, t.ms, strings.TrimSuffix(t.path, "index.html"), t.path))
	}
	data.Message += `</tbody></table>`
}
