package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveDelete(db serverDB, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)

	boardDir := FormString(r, "board")
	b := db.BoardByDir(boardDir)
	if b == nil {
		data.BoardError(w, data.G("No board specified."))
		return
	}
	data.Board = b

	subscribe := FormString(r, "subscribe") != ""

	var posts []*Post
	for _, formValue := range r.Form["delete[]"] {
		postID, err := strconv.Atoi(formValue)
		if err == nil && postID > 0 {
			post := db.PostByID(postID)
			if post != nil {
				posts = append(posts, post)
			}
		}
	}

	if subscribe {
		if len(posts) == 0 {
			threadID := FormInt(r, "thread")
			if threadID > 0 {
				post := db.PostByID(threadID)
				if post != nil {
					posts = append(posts, post)
				}
			}
		}
		if len(posts) == 0 {
			data.BoardError(w, data.G("Invalid or deleted post."))
			return
		}
		url := fmt.Sprintf("/sriracha/subscribe/post/%d", posts[0].ID)
		data.Redirect(w, r, url)
		return
	} else if data.Account != nil {
		if len(posts) > 0 {
			url := "/sriracha/mod/"
			for i, post := range posts {
				if i != 0 {
					url += ","
				}
				url += strconv.Itoa(post.ID)
			}
			data.Redirect(w, r, url)
			return
		} else if FormString(r, "moderate") != "" {
			data.BoardError(w, Get(b, data.Account, "No post selected."))
			return
		}

		threadID := FormInt(r, "thread")
		if threadID > 0 {
			post := db.PostByID(threadID)
			if post != nil {
				posts = append(posts, post)
			}
		}
		url := fmt.Sprintf("/sriracha/board/mod/%d", b.ID)
		if len(posts) > 0 {
			post := posts[0]
			url += fmt.Sprintf("/%d#%d", post.Thread(), post.ID)
		}
		data.Redirect(w, r, url)
		return
	} else if len(posts) > 0 {
		post := posts[0]
		if post.Archived() {
			data.BoardError(w, Get(b, data.Account, "Only active posts may be deleted."))
			return
		}

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
			s.pageTimingLock.Lock()
			delete(s.pageTimings, b.Path()+fmt.Sprintf("res/%d.html", post.ID))
			s.pageTimingLock.Unlock()
		} else {
			s.unBumpThread(db, post.Parent)
		}

		wg := &sync.WaitGroup{}
		delta := &atomic.Uint32{}
		db.SoftCommit()
		s.rebuildThread(db, wg, delta, post)
		wg.Wait()

		data.Template = "board_info"
		data.Info = data.Get("Deleted %s.", fmt.Sprintf("No.%d", post.ID))
		data.execute(w)
		return
	}
	data.BoardError(w, Get(b, data.Account, "No post selected."))
}
