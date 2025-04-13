package sriracha

import (
	"net/http"
)

func (s *Server) servePlugin(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_plugin"
	data.Manage.Plugins = allPluginInfo
}
