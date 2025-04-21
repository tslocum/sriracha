package sriracha

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"
)

func (s *Server) serveReport(db *Database, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)

	postID := formInt(r, "post")
	if postID <= 0 {
		data.BoardError(w, "No post selected.")
		return
	}

	b := db.boardByID(formInt(r, "board"))
	if b == nil {
		debug.PrintStack()
		data.BoardError(w, "No board was specified.")
		return
	}

	post := db.postByID(b, postID)
	if post == nil {
		data.BoardError(w, "No post selected.")
		return
	} else if post.Moderated == ModeratedVisible {
		report := &Report{
			Board:     b,
			Post:      post,
			Timestamp: time.Now().Unix(),
			IP:        hashIP(r.RemoteAddr),
		}
		db.addReport(report)
	}

	data.Template = "board_info"
	data.Info = fmt.Sprintf("Reported No.%d", post.ID)
	data.execute(w)
}
