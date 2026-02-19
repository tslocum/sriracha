package server

import (
	"fmt"
	"net/http"
	"time"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveSubscribe(db *database.DB, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)
	data.Template = "subscribe"
	data.Boards = db.AllBoards()

	key := r.URL.Query().Get("key")
	if key != "" {
		email := r.URL.Query().Get("email")
		if email == "" {
			data.BoardError(w, "Invalid email.")
			return
		}
		expectedKey := md5Sum(s.hashData(md5Sum(email)))
		if key != expectedKey {
			data.BoardError(w, "Invalid access key.")
			return
		}
		data.Extra = email
		data.execute(w)
		return
	}

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

	if data.Post == nil && data.Board == nil {
		http.Redirect(w, r, "/sriracha/", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		email := FormString(r, "email")
		if email == "" {
			data.BoardError(w, "Enter your email address to subscribe.")
			return
		}
		s := &Subscription{
			IP:      s.hashIP(r),
			Confirm: time.Now().Unix(), // TODO
			Email:   email,
		}
		if data.Post != nil {
			s.Target = data.Post.ID
		} else {
			s.Board = data.Board.ID
			s.Target = int(FormRange(r, "notify", SubscriptionThreads, SubscriptionAll))
		}
		err := s.Validate()
		if err != nil {
			data.BoardError(w, fmt.Sprintf("Failed to add subscription: %s", err))
			return
		}
		db.AddSubscription(s)

		var target string
		if data.Post != nil {
			target = fmt.Sprintf("No.%d", data.Post.ID)
		} else {
			target = data.Board.Path()
		}

		data.Template = "board_info"
		data.Info = fmt.Sprintf("Subscribed to %s", target)
	}

	data.execute(w)
}
