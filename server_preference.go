package sriracha

import (
	"net/http"
	"strings"
)

func (s *Server) servePreference(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_preference"
	if r.Method == http.MethodPost {
		switch formString(r, "action") {
		case "style":
			var style string
			if formString(r, "style") == "burichan" || formString(r, "style") == "sriracha" {
				style = formString(r, "style")
			}
			db.updateAccountStyle(data.Account.ID, style)

			http.Redirect(w, r, "/sriracha/preference/", http.StatusFound)
			return
		case "password":
			oldPass := r.FormValue("old")
			newPass := r.FormValue("new")
			confirmPass := r.FormValue("confirmation")
			if strings.TrimSpace(oldPass) == "" || strings.TrimSpace(newPass) == "" || strings.TrimSpace(confirmPass) == "" {
				data.ManageError("All fields are required")
				return
			}

			if newPass != confirmPass {
				data.ManageError("New passwords do not match")
				return
			}

			match := db.loginAccount(data.Account.Username, oldPass)
			if match == nil {
				data.ManageError("Current password is incorrect")
				return
			}

			db.updateAccountPassword(match.ID, newPass)

			http.Redirect(w, r, "/sriracha/", http.StatusFound)
			return
		}
	}
}
