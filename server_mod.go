package sriracha

import (
	"net/http"
	"strings"
)

func (s *Server) serveMod(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_mod"

	var boardID int
	var postID int
	var action = "db"
	modInfo := pathString(r, "/sriracha/mod/")
	if modInfo != "" {
		split := strings.Split(modInfo, "/")
		if len(split) == 3 {
			switch split[0] {
			case "delete":
				action = "d"
			case "ban":
				action = "n"
			default:
				data.ManageError("Unknown mod action")
				return
			}
			boardID = parseInt(split[1])
			postID = parseInt(split[2])
		} else if len(split) == 2 {
			boardID = parseInt(split[0])
			postID = parseInt(split[1])
		}
	}
	if boardID == 0 || postID == 0 {
		data.ManageError("Unknown board or post")
		return
	}
	data.Board = db.boardByID(boardID)
	data.Post = db.postByID(data.Board, postID)
	if data.Board == nil || data.Post == nil {
		data.ManageError("Unknown board or post")
		return
	}
	data.Manage.Ban = db.banByIP(data.Post.IP)
	data.Extra = action
}
