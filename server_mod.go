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
				action = "n"
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
	data.Post = db.postByID(postID)
	if data.Post == nil {
		data.ManageError("Unknown post")
		return
	}
	data.Board = data.Post.Board
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

			s.rebuildThread(db, data.Board, data.Post)
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
