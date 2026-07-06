package server

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveDelete(db serverDB, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)

	boardDir := FormString(r, "board")
	b := db.BoardByDir(boardDir)
	if b == nil {
		data.BoardError(w, Get(b, data.Account, "No board specified."))
		return
	}

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
			data.BoardError(w, Get(b, data.Account, "Invalid post."))
			return
		}
		url := fmt.Sprintf("/sriracha/subscribe/post/%d", posts[0].ID)
		data.Redirect(w, r, url)
		return
	} else if data.Account != nil {
		if len(posts) > 1 {
			var actionDelete, actionBan bool
			switch {
			case FormString(r, "bulkd") != "":
				actionDelete = true
			case FormString(r, "bulkb") != "":
				actionBan = true
			case FormString(r, "bulkdb") != "":
				actionDelete = true
				actionBan = true
			}
			if !actionDelete && !actionBan {
				data.Template = "manage_begin"
				data.execute(w)
				w.Write([]byte(`<h2 class="managetitle">` + GetN(nil, data.Account, "Moderate %d post", "Moderate %d posts", len(posts)) + `</h2>
				<form method="post" action="/sriracha/">
				<input type="hidden" name="action" value="delete">
				<input type="hidden" name="board" value="` + template.HTMLEscapeString(posts[0].Board.Dir) + `">`))
				for _, post := range posts {
					w.Write([]byte(`<input type="hidden" name="delete[]" value="` + strconv.Itoa(post.ID) + `">`))
				}
				w.Write([]byte(`<input type="submit" name="bulkd" value="` + G(nil, data.Account, "Delete") + `" onclick="return confirm('` + GetN(nil, data.Account, "Delete %d post?", "Delete %d posts?", len(posts)) + `')"> <input type="submit" name="bulkb" value="` + G(nil, data.Account, "Ban") + `"> <input type="submit" name="bulkdb" value="` + G(nil, data.Account, "Delete & ban") + `">
				</form><br>
				<hr><br>`))
				for _, post := range posts {
					if post.Board.Type == TypeImageboard {
						data.Template = "imgboard_post"
					} else {
						data.Template = "forum_post"
					}
					data.ReplyMode = post.Thread()
					data.Board = post.Board
					data.Threads = [][]*Post{{post}}
					data.execute(w)
				}
				data.Template = "manage_end"
				data.execute(w)
				return
			} else if !actionBan {
				for _, post := range posts {
					s.deletePost(db, post)
					if post.Parent == 0 {
						os.Remove(filepath.Join(s.config.Root, post.Board.Dir, "res", fmt.Sprintf("%d.html", post.ID)))
					} else {
						s.writeThread(db, post.Board, post.Thread())
					}
				}
				s.writeBoardIndexes(db, posts[0].Board)
				s.writeSiteIndex(db)

				data.Template = "board_info"
				data.Info = fmt.Sprintf("Deleted %d posts", len(posts))
				data.execute(w)
				return
			}
			// Ban
			return
		}

		if len(posts) == 0 {
			threadID := FormInt(r, "thread")
			if threadID > 0 {
				post := db.PostByID(threadID)
				if post != nil {
					posts = append(posts, post)
				}
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
		s.writeBoardIndexes(db, b)

		data.Template = "board_info"
		data.Info = fmt.Sprintf("Deleted No.%d", post.ID)
		data.execute(w)
		return
	}
	data.BoardError(w, Get(b, data.Account, "No post selected."))
}
