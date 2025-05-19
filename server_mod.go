package sriracha

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) serveMod(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_mod"

	var postID int
	var action = "db"
	modInfo := pathString(r, "/sriracha/mod/")
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
			default:
				data.ManageError("Unknown mod action")
				return
			}
			postID = parseInt(split[1])
		} else if len(split) == 1 {
			postID = parseInt(split[0])
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
	threadAction := action == "s" || action == "us" || action == "l" || action == "ul"
	if threadAction {
		if data.Post.Parent != 0 {
			data.ManageError("Invalid post")
			return
		}

		var skipRebuild bool
		switch {
		case action == "s" && !data.Post.Stickied:
			db.stickyPost(data.Post.ID, true)
			db.log(data.Account, nil, fmt.Sprintf("Stickied >>/post/%d", data.Post.ID), "")
		case action == "us" && data.Post.Stickied:
			db.stickyPost(data.Post.ID, false)
			db.log(data.Account, nil, fmt.Sprintf("Unstickied >>/post/%d", data.Post.ID), "")
		case action == "l" && !data.Post.Locked:
			db.lockPost(data.Post.ID, true)
			db.log(data.Account, nil, fmt.Sprintf("Locked >>/post/%d", data.Post.ID), "")
		case action == "ul" && data.Post.Locked:
			db.lockPost(data.Post.ID, false)
			db.log(data.Account, nil, fmt.Sprintf("Unlocked >>/post/%d", data.Post.ID), "")
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
	data.Manage.Ban = db.banByIP(data.Post.IP)
	if r.FormValue("confirmation") == "1" {
		var oldBan Ban
		if data.Manage.Ban != nil {
			oldBan = *data.Manage.Ban
		}
		if action == "b" || action == "db" {
			if data.Manage.Ban != nil {
				data.Manage.Ban.loadForm(r)
				db.updateBan(data.Manage.Ban)

				changes := printChanges(oldBan, *data.Manage.Ban)
				db.log(data.Account, nil, fmt.Sprintf("Updated >>/ban/%d", data.Manage.Ban.ID), changes)
			} else {
				ban := &Ban{}
				ban.loadForm(r)
				ban.IP = data.Post.IP
				db.addBan(ban)

				db.log(data.Account, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info())
			}
		}
		if action == "d" || action == "db" {
			s.deletePost(db, data.Post)

			db.log(data.Account, data.Board, fmt.Sprintf("Deleted No.%d", data.Post.ID), "")

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

	data.Extra = action
}
