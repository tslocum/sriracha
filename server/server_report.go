package server

import (
	"log"
	"net/http"
	"time"

	"codeberg.org/tslocum/sriracha/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveReport(db *database.DB, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)

	postID := FormInt(r, "post")
	if postID <= 0 {
		data.BoardError(w, Get(nil, data.Account, "No post selected."))
		return
	}

	post := db.PostByID(postID)
	if post == nil {
		data.BoardError(w, Get(nil, data.Account, "No post selected."))
		return
	} else if post.Moderated == ModeratedVisible {
		numReports := db.NumReports(post)
		if numReports == 0 {
			postCopy := post.Copy()
			for _, info := range allPluginReportHandlers {
				db.Plugin = info.Name
				err := info.Handler(db, postCopy)
				if err != nil {
					log.Fatalf("plugin %s failed to process report event: %s", info.Name, err)
				}
			}
			db.Plugin = ""
		}

		report := &Report{
			Board:     post.Board,
			Post:      post,
			Timestamp: time.Now().Unix(),
			IP:        s.hashIP(r),
		}
		db.AddReport(report)
	}

	data.Template = "board_info"
	data.Info = Get(post.Board, data.Account, "Reported No.%d", post.ID)
	data.execute(w)
}
