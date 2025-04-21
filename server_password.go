package sriracha

import (
	"net/http"
	"strings"
)

func (s *Server) serveChangePassword(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	oldPass := r.FormValue("old")
	newPass := r.FormValue("new")
	confirmPass := r.FormValue("confirmation")
	data.Template = "manage_password"
	if r.Method != http.MethodPost {
		return
	} else if strings.TrimSpace(oldPass) == "" || strings.TrimSpace(newPass) == "" || strings.TrimSpace(confirmPass) == "" {
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
}
