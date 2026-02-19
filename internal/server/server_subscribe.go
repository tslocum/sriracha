package server

import (
	"net/http"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveSubscribe(db *database.DB, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)
	data.Template = "subscribe"
	data.Boards = db.AllBoards()

	boardID := PathInt(r, "/sriracha/subscribe/board/")
	if boardID > 0 {
		board := db.BoardByID(boardID)
		if board == nil {
			data.BoardError(w, "Invalid board.")
			return
		}
		data.Board = board
	} else {
		postID := PathInt(r, "/sriracha/subscribe/post/")
		if postID > 0 {
			post := db.PostByID(postID)
			if post == nil {
				data.BoardError(w, "Invalid post.")
				return
			}
			data.Board = post.Board
			data.Post = post
		}
	}

	data.execute(w)
}
