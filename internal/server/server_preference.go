package server

import (
	"net/http"
	"slices"
	"strings"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) servePreference(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_preference"
	if r.Method == http.MethodPost {
		switch FormString(r, "action") {
		case "style":
			var style string
			if FormString(r, "style") == "burichan" || FormString(r, "style") == "sriracha" {
				style = FormString(r, "style")
			}
			db.UpdateAccountStyle(data.Account.ID, style)

			http.Redirect(w, r, "/sriracha/preference/", http.StatusFound)
			return
		case "locale":
			locale := FormString(r, "locale")
			if locale != "" && !slices.Contains(s.opt.LocalesSorted, locale) {
				locale = ""
			}
			db.UpdateAccountLocale(data.Account.ID, locale)

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

			match := db.LoginAccount(data.Account.Username, oldPass)
			if match == nil {
				data.ManageError("Current password is incorrect")
				return
			}

			db.UpdateAccountPassword(match.ID, newPass)

			http.Redirect(w, r, "/sriracha/", http.StatusFound)
			return
		}
	}
}
