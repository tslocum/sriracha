package sriracha

import (
	"net/http"
)

func (s *Server) serveLog(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_log"
	data.Manage.Logs = db.allLogs()
}
