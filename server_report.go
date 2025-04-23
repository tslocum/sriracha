package sriracha

import (
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

	post := db.postByID(postID)
	if post == nil {
		data.BoardError(w, gotext.Get("No post selected."))
		return
	} else if post.Moderated == ModeratedVisible {
		report := &Report{
			Board:     post.Board,
			Post:      post,
			Timestamp: time.Now().Unix(),
			IP:        hashIP(r.RemoteAddr),
		}
		db.addReport(report)
	}

	data.Template = "board_info"
	data.Info = gotext.Get("Reported No.%d", post.ID)
	data.execute(w)
}
