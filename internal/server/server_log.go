package server

import (
	"net/http"

	. "codeberg.org/tslocum/sriracha/util"
)

const logPageSize = 25

func (s *Server) serveLog(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	page := PathInt(r, "/sriracha/log/p")
	data.Template = "manage_log"
	data.Manage.Logs = db.LogsByPage(page)
	data.Page = page
	data.Pages = pageCount(db.LogCount(), logPageSize)
}
