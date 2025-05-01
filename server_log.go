package sriracha

import (
	"net/http"
)

const logPageSize = 25

func (s *Server) serveLog(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	page := pathInt(r, "/sriracha/log/p")
	data.Template = "manage_log"
	data.Manage.Logs = db.logsByPage(page)
	data.Page = page
	data.Pages = pageCount(db.logCount(), logPageSize)
}
