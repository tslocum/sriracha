package server

import "net/http"

func (s *Server) serveTwoFactor(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_twofactor"
}
