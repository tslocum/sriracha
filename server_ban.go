package sriracha

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

func (s *Server) serveBan(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_ban"
	data.Boards = db.AllBoards()

	deleteBanID := pathInt(r, "/sriracha/ban/delete/")
	if deleteBanID > 0 {
		b := db.banByID(deleteBanID)
		if b == nil {
			data.ManageError("Invalid ban.")
			return
		}
		db.deleteBan(b.ID)

		if strings.HasPrefix(b.IP, "r ") {
			s.reloadBans(db)
		}

		var changes string
		liftReason := formString(r, "reason")
		if strings.TrimSpace(liftReason) != "" {
			changes = "Reason: " + liftReason
		}

		db.log(data.Account, nil, fmt.Sprintf("Lifted >>/ban/%d", b.ID), changes)

		http.Redirect(w, r, "/sriracha/ban/", http.StatusFound)
		return
	}

	banID := pathInt(r, "/sriracha/ban/")
	if banID > 0 {
		data.Manage.Ban = db.banByID(banID)

		if data.Manage.Ban != nil && r.Method == http.MethodPost {
			oldBan := *data.Manage.Ban
			data.Manage.Ban.loadForm(r)

			shorter := data.Manage.Ban.Expire != 0 && (oldBan.Expire == 0 || data.Manage.Ban.Expire < oldBan.Expire)
			if shorter && data.forbidden(w, RoleAdmin) {
				return
			}

			err := data.Manage.Ban.validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			db.updateBan(data.Manage.Ban)

			if strings.HasPrefix(data.Manage.Ban.IP, "r ") {
				s.reloadBans(db)
			}

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
		if strings.ContainsRune(ip, '*') {
			pattern := strings.ReplaceAll(strings.ReplaceAll(ip, ".", `\.`), "*", ".*")
			_, err := regexp.Compile(pattern)
			if err != nil {
				data.ManageError(fmt.Sprintf("failed to compile ban `%s` as regular expression: %s", pattern, err))
				return
			}
			b.IP = "r " + pattern
		} else if ip != "" {
			b.IP = _hashIP(ip)
		}

		err := b.validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		match := db.banByIP(b.IP)
		if match != nil {
			data.ManageError("A ban for that IP address or range already exists.")
			return
		}

		db.addBan(b)

		if strings.HasPrefix(b.IP, "r ") {
			s.reloadBans(db)
		}

		db.log(data.Account, nil, fmt.Sprintf("Added >>/ban/%d", b.ID), b.Info())

		http.Redirect(w, r, "/sriracha/ban/", http.StatusFound)
		return
	}

	data.Manage.Bans = db.allBans(false)
}
