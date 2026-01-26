package server

import (
	"fmt"
	"net/http"
	"strings"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadAccountForm(db *database.DB, r *http.Request, a *Account) {
	a.Username = FormString(r, "username")
	a.Role = FormRange(r, "role", RoleSuperAdmin, RoleDisabled)
}

func (s *Server) serveAccount(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleSuperAdmin) {
		return
	}
	data.Template = "manage_account"

	accountID := PathInt(r, "/sriracha/account/")
	if accountID > 0 {
		data.Manage.Account = db.AccountByID(accountID)

		if data.Manage.Account != nil && r.Method == http.MethodPost {
			oldAccount := *data.Manage.Account
			oldUsername := data.Manage.Account.Username
			s.loadAccountForm(db, r, data.Manage.Account)

			err := data.Manage.Account.Validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			if data.Account.ID == data.Manage.Account.ID && data.Manage.Account.Role != RoleSuperAdmin {
				data.ManageError("You may not change the role of your own account.")
				return
			}

			if data.Manage.Account.Username != oldUsername {
				match := db.AccountByUsername(data.Manage.Account.Username)
				if match != nil {
					data.ManageError("New username already taken")
					return
				}

				db.UpdateAccountUsername(data.Manage.Account)
			}

			db.UpdateAccountRole(data.Manage.Account)

			password := r.FormValue("password")
			if strings.TrimSpace(password) != "" {
				db.UpdateAccountPassword(data.Manage.Account.ID, password)
			}

			changes := printChanges(oldAccount, *data.Manage.Account)
			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/account/%d", data.Manage.Account.ID), changes)

			http.Redirect(w, r, "/sriracha/account/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		a := &Account{}
		s.loadAccountForm(db, r, a)

		err := a.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		password := r.FormValue("password")
		if strings.TrimSpace(password) == "" {
			data.ManageError("A password is required")
			return
		}

		db.AddAccount(a, password)

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/account/%d", a.ID), "")

		http.Redirect(w, r, "/sriracha/account/", http.StatusFound)
		return
	}

	data.Manage.Accounts = db.AllAccounts()
}
