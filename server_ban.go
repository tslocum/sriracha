package sriracha

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) serveBan(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	var err error
	data.Template = "manage_ban"
	data.Boards, err = db.allBoards()
	if err != nil {
		log.Fatal(err)
	}

	banID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/imgboard/ban/"))
	if err == nil && banID > 0 {
		data.Manage.Ban, err = db.banByID(banID)
		if err != nil {
			log.Fatal(err)
		}

		if data.Manage.Ban != nil && r.Method == http.MethodPost {
			data.Manage.Ban.loadForm(r)

			err := data.Manage.Ban.validate()
			if err != nil {
				data.Error(err.Error())
				return
			}

			err = db.updateBan(data.Manage.Ban)
			if err != nil {
				data.Error(err.Error())
				return
			}

			err = db.log(data.Account, nil, fmt.Sprintf("Updated >>/ban/%d", data.Manage.Ban.ID))
			if err != nil {
				log.Fatal(err)
			}
			http.Redirect(w, r, "/imgboard/ban/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		b := &Ban{}
		b.loadForm(r)

		ip := strings.TrimSpace(r.FormValue("ip"))
		if ip != "" {
			b.IP = db.encryptPassword(ip)
		}

		err := b.validate()
		if err != nil {
			data.Error(err.Error())
			return
		}

		match, err := db.banByIP(b.IP)
		if err != nil {
			log.Fatal(err)
		} else if match != nil {
			data.Error("Ban text already exists")
			return
		}

		err = db.addBan(b)
		if err != nil {
			log.Fatal(err)
			return
		}

		err = db.log(data.Account, nil, fmt.Sprintf("Added >>/ban/%d", b.ID))
		if err != nil {
			log.Fatal(err)
		}
		http.Redirect(w, r, "/imgboard/ban/", http.StatusFound)
		return
	}

	data.Manage.Bans, err = db.allBans()
	if err != nil {
		log.Fatal(err)
	}
}
