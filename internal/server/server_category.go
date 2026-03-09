package server

import (
	"net/http"

	"codeberg.org/tslocum/sriracha/internal/database"
)

func (s *Server) serveCategory(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_category"
}
