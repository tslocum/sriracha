package sriracha

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) serveAccount(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_account"

	accountID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/imgboard/account/"))
	if err == nil && accountID > 0 {
		data.Manage.Account, err = db.accountByID(accountID)
		if err != nil {
			log.Fatal(err)
		}

		if data.Manage.Account != nil && r.Method == http.MethodPost {
			oldUsername := data.Manage.Account.Username
			data.Manage.Account.loadForm(r)

			err := data.Manage.Account.validate()
			if err != nil {
				data.Error(err.Error())
				return
			}

			if data.Manage.Account.Username != oldUsername {
				match, err := db.accountByUsername(data.Manage.Account.Username)
				if err != nil {
					log.Fatal(err)
				} else if match != nil {
					data.Error("New username already taken")
					return
				}

				err = db.updateAccountUsername(data.Manage.Account)
				if err != nil {
					data.Error(err.Error())
					return
				}
			}

			err = db.updateAccountRole(data.Manage.Account)
			if err != nil {
				data.Error(err.Error())
				return
			}

			password := r.FormValue("password")
			if strings.TrimSpace(password) != "" {
				err = db.updateAccountPassword(data.Manage.Account.ID, password)
				if err != nil {
					data.Error(err.Error())
					return
				}
			}

			http.Redirect(w, r, "/imgboard/account/", http.StatusFound)
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

		err = db.addAccount(a, password)
		if err != nil {
			data.Error(err.Error())
			return
		}
	}

	data.Manage.Accounts, err = db.allAccounts()
	if err != nil {
		log.Fatal(err)
	}
}
