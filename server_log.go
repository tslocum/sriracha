package sriracha

import (
	"log"
	"net/http"
)

func (s *Server) serveLog(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	var err error
	data.Template = "manage_log"
	data.Manage.Logs, err = db.allLogs()
	if err != nil {
		log.Fatal(err)
	}
}
