package server

import (
	"fmt"
	"net/http"
	"strings"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveMod(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_mod"

	var postID int
	var action = "db"
	modInfo := PathString(r, "/sriracha/mod/")
	if modInfo != "" {
		split := strings.Split(modInfo, "/")
		if len(split) == 2 {
			switch split[0] {
			case "delete":
				action = "d"
			case "ban":
				action = "b"
			case "sticky":
				action = "s"
			case "unsticky":
				action = "us"
			case "lock":
				action = "l"
			case "unlock":
				action = "ul"
			case "view":
				action = "v"
			default:
				data.ManageError("Unknown mod action")
				return
			}
			postID = ParseInt(split[1])
		} else if len(split) == 1 {
			postID = ParseInt(split[0])
		}
	}
	if postID == 0 {
		data.ManageError("Unknown post")
		return
	}
	data.Post = db.PostByID(postID)
	if data.Post == nil {
		data.ManageError("Unknown post")
		return
	}
	if action == "v" {
		if !s.opt.Identifiers {
			data.ManageError("Identifiers are not enabled")
			return
		}
		data.Template = "board_page"
		data.ModMode = true
		data.ReplyMode = 1
		data.Board = data.Post.Board
		posts := db.PostsByIP(data.Post.IP)
		if r.FormValue("confirmation") == "1" {
			for _, post := range posts {
				s.deletePost(db, post)
				s.log(db, data.Account, post.Board, fmt.Sprintf("Deleted No.%d", post.ID), "")
				s.rebuildThread(db, post)
			}
			data.Message = "Deleted all posts by author."
			return
		}
		for _, post := range posts {
			data.Threads = append(data.Threads, []*Post{post})
		}
		data.Message = `<form method="post" onsubmit="javascript:return confirm('Delete all posts by author?');"><input type="hidden" name="confirmation" value="1"><input type="submit" value="Delete all posts by author"></form><br>`
		return
	}
	threadAction := action == "s" || action == "us" || action == "l" || action == "ul"
	if threadAction {
		if data.Post.Parent != 0 {
			data.ManageError("Invalid post")
			return
		}

		var skipRebuild bool
		switch {
		case action == "s" && !data.Post.Stickied:
			db.StickyPost(data.Post.ID, true)
			s.log(db, data.Account, nil, fmt.Sprintf("Stickied >>/post/%d", data.Post.ID), "")
		case action == "us" && data.Post.Stickied:
			db.StickyPost(data.Post.ID, false)
			s.log(db, data.Account, nil, fmt.Sprintf("Unstickied >>/post/%d", data.Post.ID), "")
		case action == "l" && !data.Post.Locked:
			db.LockPost(data.Post.ID, true)
			s.log(db, data.Account, nil, fmt.Sprintf("Locked >>/post/%d", data.Post.ID), "")
		case action == "ul" && data.Post.Locked:
			db.LockPost(data.Post.ID, false)
			s.log(db, data.Account, nil, fmt.Sprintf("Unlocked >>/post/%d", data.Post.ID), "")
		default:
			skipRebuild = true
		}
		if !skipRebuild {
			s.rebuildThread(db, data.Post)
		}

		data.Template = "manage_info"
		http.Redirect(w, r, fmt.Sprintf("/sriracha/board/mod/%d/%d", data.Post.Board.ID, data.Post.ID), http.StatusFound)
		return
	}
	data.Board = data.Post.Board
	data.Threads = [][]*Post{{data.Post}}
	data.Manage.Ban = db.BanByIP(data.Post.IP)
	if r.FormValue("confirmation") == "1" {
		banFile := FormString(r, "banfile")
		if banFile != "" && !db.FileBanned(banFile) {
			db.AddFileBan(banFile)
			s.log(db, data.Account, nil, "Banned file", "")
		}

		var oldBan Ban
		if data.Manage.Ban != nil {
			oldBan = *data.Manage.Ban
		}
		if action == "b" || action == "db" {
			if data.Manage.Ban != nil {
				s.loadBanForm(db, r, data.Manage.Ban)
				db.UpdateBan(data.Manage.Ban)

				changes := printChanges(oldBan, *data.Manage.Ban)
				s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/ban/%d", data.Manage.Ban.ID), changes)
			} else {
				ban := &Ban{}
				s.loadBanForm(db, r, ban)
				ban.IP = data.Post.IP
				db.AddBan(ban)

				s.log(db, data.Account, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info())
			}
		}
		if action == "d" || action == "db" {
			s.deletePost(db, data.Post)

			s.log(db, data.Account, data.Board, fmt.Sprintf("Deleted No.%d", data.Post.ID), "")

			s.rebuildThread(db, data.Post)
		}

		label := "Deleted"
		switch action {
		case "b":
			label = "Banned"
		case "db":
			label = "Deleted and banned"
		}

		data.Template = "manage_info"
		data.Info = fmt.Sprintf("%s No.%d", label, data.Post.ID)
		return
	}

	data.ModMode = true
	data.ReplyMode = 1
	data.Extra = action
	if data.Post != nil {
		data.Extra2 = data.Post.FileHash
	}
}
