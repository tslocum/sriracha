package server

import (
	"net/http"

	"codeberg.org/tslocum/sriracha/database"
	. "codeberg.org/tslocum/sriracha/util"
)

const logPageSize = 25

func (s *Server) serveLog(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	page := PathInt(r, "/sriracha/log/p")
	data.Template = "manage_log"
	data.Manage.Logs = db.LogsByPage(page)
	data.Page = page
	data.Pages = pageCount(db.LogCount(), logPageSize)
}
