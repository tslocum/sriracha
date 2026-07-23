package server

import (
	"bytes"
	"html/template"
	"net/http"
	"slices"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

const searchPageSize = 10

func (s *Server) serveSearch(db serverDB, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)
	if s.opt.Search == 0 && data.forbidden(w, RoleMod) {
		data.execute(w)
		return
	}
	data.Template = "search"
	data.Boards = db.AllBoards()
	data.Categories = db.AllCategories()
	query := FormString(r, "q")
	if query != "" {
		var boards []*Board
		if data.Account == nil {
			now := time.Now().Unix()
			ipHash := s.hashIP(r)
			since := now - s.lastSearch[ipHash]
			if since < int64(s.opt.Search) {
				data.BoardError(w, Get(nil, data.Account, "Please wait %s before searching again.", time.Duration(int64(s.opt.Search)-since)*time.Second))
				return
			}
			s.lastSearch[ipHash] = now
			for _, c := range s.opt.Categories {
				for _, b := range c.Boards {
					if !slices.Contains(boards, b) {
						boards = append(boards, b)
					}
				}
			}
		}
		data.Extra3 = query
		results := db.SearchPosts(query, boards...)
		data.Page = FormInt(r, "p")
		data.Pages = pageCount(len(results), searchPageSize)
		results = pageSlice(results, data.Page, searchPageSize)
		if len(results) == 0 {
			data.Message = "<br>" + GetHTML(nil, data.Account, "No matching posts were found.") + "<br><br><hr>"
		} else {
			data.Message = "<br>"
			out := &bytes.Buffer{}
			subData := s.buildData(db, w, r)
			subData.Template = "imgboard_post"
			for _, id := range results {
				post := db.PostByID(id)
				if post == nil {
					continue
				}
				subData.Board = post.Board
				subData.Threads = [][]*Post{{post}}
				subData.execute(out)
				data.Message += template.HTML(out.String())
				out.Reset()
			}
			data.Message += "<br><hr>"
		}
	}
	data.execute(w)
}
