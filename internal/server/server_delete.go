package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveDelete(db *database.DB, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)

	boardDir := FormString(r, "board")
	b := db.BoardByDir(boardDir)
	if b == nil {
		data.BoardError(w, Get(b, data.Account, "No board specified."))
		return
	}

	var post *Post
	postID, err := strconv.Atoi(r.FormValue("delete[]"))
	if err == nil && postID > 0 {
		post = db.PostByID(postID)
	}
	if data.Account != nil {
		if post == nil {
			threadID := FormInt(r, "thread")
			if threadID > 0 {
				post = db.PostByID(threadID)
			}
		}
		url := fmt.Sprintf("/sriracha/board/mod/%d", b.ID)
		if post != nil {
			url += fmt.Sprintf("/%d#%d", post.Thread(), post.ID)
		}
		http.Redirect(w, r, url, http.StatusFound)
		return
	} else if post != nil {
		password := r.FormValue("password")
		if post.Password == "" || s.hashData(password) != post.Password {
			data.BoardError(w, Get(b, data.Account, "Incorrect password."))
			return
		}

		confirm := r.FormValue("confirmation")
		if confirm != "1" {
			data.Board = b
			data.Post = post
			data.Extra = password
			data.Template = "board_delete"
			data.execute(w)
			return
		}

		s.deletePost(db, post)

		if post.Parent == 0 {
			os.Remove(filepath.Join(s.config.Root, b.Dir, "res", fmt.Sprintf("%d.html", post.ID)))
		} else {
			s.writeThread(db, b, post.Thread())
		}
		s.writeIndexes(db, b)

		data.Template = "board_info"
		data.Info = fmt.Sprintf("Deleted No.%d", post.ID)
		data.execute(w)
		return
	}
	data.BoardError(w, Get(b, data.Account, "No post selected."))
}
