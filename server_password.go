package sriracha

import (
	"log"
	"net/http"
	"strings"
)

func (s *Server) serveChangePassword(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	oldPass := r.FormValue("old")
	newPass := r.FormValue("new")
	confirmPass := r.FormValue("confirm")
	data.Template = "manage_password"
	if r.Method != http.MethodPost {
		return
	} else if strings.TrimSpace(oldPass) == "" || strings.TrimSpace(newPass) == "" || strings.TrimSpace(confirmPass) == "" {
		data.Error("All fields are required")
		return
	}

	if newPass != confirmPass {
		data.Error("New passwords do not match")
		return
	}

	match, err := db.loginAccount(data.Account.Username, oldPass)
	if err != nil {
		log.Fatal(err)
	} else if match == nil {
		data.Error("Current password is incorrect")
		return
	}

	err = db.updateAccountPassword(match.ID, newPass)
	if err != nil {
		log.Fatal(err)
	}

	http.Redirect(w, r, "/imgboard/", http.StatusFound)
}
