package sriracha

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) serveAccount(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_account"

	accountID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/account/"))
	if err == nil && accountID > 0 {
		data.Manage.Account = db.accountByID(accountID)

		if data.Manage.Account != nil && r.Method == http.MethodPost {
			oldAccount := *data.Manage.Account
			oldUsername := data.Manage.Account.Username
			data.Manage.Account.loadForm(r)

			err := data.Manage.Account.validate()
			if err != nil {
				data.Error(err.Error())
				return
			}

			if data.Manage.Account.Username != oldUsername {
				match := db.accountByUsername(data.Manage.Account.Username)
				if match != nil {
					data.Error("New username already taken")
					return
				}

				db.updateAccountUsername(data.Manage.Account)
			}

			db.updateAccountRole(data.Manage.Account)

			password := r.FormValue("password")
			if strings.TrimSpace(password) != "" {
				db.updateAccountPassword(data.Manage.Account.ID, password)
			}

			changes := printChanges(oldAccount, *data.Manage.Account)
			db.log(data.Account, nil, fmt.Sprintf("Updated >>/account/%d", data.Manage.Account.ID), changes)

			http.Redirect(w, r, "/sriracha/account/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		a := &Account{}
		a.loadForm(r)

		err := a.validate()
		if err != nil {
			data.Error(err.Error())
			return
		}

		password := r.FormValue("password")
		if strings.TrimSpace(password) == "" {
			data.Error("A password is required")
			return
		}

		db.addAccount(a, password)

		db.log(data.Account, nil, fmt.Sprintf("Added >>/account/%d", a.ID), "")

		http.Redirect(w, r, "/sriracha/account/", http.StatusFound)
		return
	}

	data.Manage.Accounts = db.allAccounts()
}
