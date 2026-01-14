package sriracha

import (
	"log"
	"net/http"
	"time"

	"github.com/leonelquinteros/gotext"
)

func (s *Server) serveReport(db *Database, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)

	postID := formInt(r, "post")
	if postID <= 0 {
		data.BoardError(w, gotext.Get("No post selected."))
		return
	}

	post := db.PostByID(postID)
	if post == nil {
		data.BoardError(w, gotext.Get("No post selected."))
		return
	} else if post.Moderated == ModeratedVisible {
		numReports := db.numReports(post)
		if numReports == 0 {
			postCopy := post.Copy()
			for _, info := range allPluginReportHandlers {
				db.plugin = info.Name
				err := info.Handler(db, postCopy)
				if err != nil {
					log.Fatalf("plugin %s failed to process report event: %s", info.Name, err)
				}
			}
			db.plugin = ""
		}

		report := &Report{
			Board:     post.Board,
			Post:      post,
			Timestamp: time.Now().Unix(),
			IP:        hashIP(r),
		}
		db.addReport(report)
	}

	data.Template = "board_info"
	data.Info = gotext.Get("Reported No.%d", post.ID)
	data.execute(w)
}
