package sriracha

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/leonelquinteros/gotext"
)

func (s *Server) serveDelete(db *Database, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)

	boardDir := formString(r, "board")
	b := db.boardByDir(boardDir)
	if b == nil {
		data.BoardError(w, gotext.Get("No board specified."))
		return
	}

	var post *Post
	postID, err := strconv.Atoi(r.FormValue("delete[]"))
	if err == nil && postID > 0 {
		post = db.postByID(postID)
	}
	if data.Account != nil {
		url := fmt.Sprintf("/sriracha/board/mod/%d", b.ID)
		if post != nil {
			url += fmt.Sprintf("/%d#%d", post.Thread(), post.ID)
		}
		http.Redirect(w, r, url, http.StatusFound)
		return
	} else if post != nil {
		password := r.FormValue("password")
		if post.Password == "" || hashData(password) != post.Password {
			data.BoardError(w, gotext.Get("Incorrect password."))
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
	data.BoardError(w, gotext.Get("No post selected."))
}
