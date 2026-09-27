package server

import (
	"net/http"
	"slices"
	"strings"

	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) servePreference(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	s.addTwoFactorNotice(data, db)

	if strings.HasPrefix(r.URL.Path, "/sriracha/preference/2fa") {
		s.serveTwoFactor(data, db, w, r)
		return
	}

	data.Template = "manage_preference"
	data.ExtraInt = len(db.AccountSessionKeys(data.Account.ID))
	if r.Method == http.MethodPost {
		switch FormString(r, "action") {
		case "style":
			stylePreference := FormString(r, "style")
			var foundStyle bool
			for _, style := range s.config.Styles {
				if stylePreference == style || stylePreference == style+"/flex" {
					foundStyle = true
					break
				}
			}
			if !foundStyle {
				data.ManageError(data.Get("Invalid %s.", strings.ToLower(data.G("Style"))))
				return
			}
			db.UpdateAccountStyle(data.Account.ID, stylePreference)

			data.Redirect(w, r, "/sriracha/preference/")
			return
		case "locale":
			locale := FormString(r, "locale")
			if locale != "" && !slices.Contains(s.opt.LocalesSorted, locale) {
				locale = ""
			}
			db.UpdateAccountLocale(data.Account.ID, locale)

			data.Redirect(w, r, "/sriracha/preference/")
			return
		case "password":
			oldPass := r.FormValue("old")
			newPass := r.FormValue("new")
			confirmPass := r.FormValue("confirmation")
			if strings.TrimSpace(oldPass) == "" || strings.TrimSpace(newPass) == "" || strings.TrimSpace(confirmPass) == "" {
				data.ManageError(data.G("All fields are required."))
				return
			}

			if newPass != confirmPass {
				data.ManageError(data.G("Passwords do not match."))
				return
			}

			match, _ := db.LoginAccount(data.Account.Username, oldPass, false)
			if match == nil {
				data.ManageError(data.G("Incorrect password."))
				return
			}

			db.UpdateAccountPassword(match, newPass)

			data.Redirect(w, r, "/sriracha/")
			return
		}
	}
}
