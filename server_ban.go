package sriracha

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) serveBan(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	var err error
	data.Template = "manage_ban"
	data.Boards = db.allBoards()

	banID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/ban/"))
	if err == nil && banID > 0 {
		data.Manage.Ban = db.banByID(banID)

		if data.Manage.Ban != nil && r.Method == http.MethodPost {
			oldBan := *data.Manage.Ban
			data.Manage.Ban.loadForm(r)

			err := data.Manage.Ban.validate()
			if err != nil {
				data.Error(err.Error())
				return
			}

			db.updateBan(data.Manage.Ban)

			changes := printChanges(oldBan, *data.Manage.Ban)
			db.log(data.Account, nil, fmt.Sprintf("Updated >>/ban/%d", data.Manage.Ban.ID), changes)

			http.Redirect(w, r, "/sriracha/ban/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		b := &Ban{}
		b.loadForm(r)

		ip := formString(r, "ip")
		if ip != "" {
			b.IP = db.encryptPassword(ip)
		}

		err := b.validate()
		if err != nil {
			data.Error(err.Error())
			return
		}

		match := db.banByIP(b.IP)
		if match != nil {
			data.Error("Ban text already exists")
			return
		}

		db.addBan(b)

		db.log(data.Account, nil, fmt.Sprintf("Added >>/ban/%d", b.ID), "")

		http.Redirect(w, r, "/sriracha/ban/", http.StatusFound)
		return
	}

	data.Manage.Bans = db.allBans()
}
