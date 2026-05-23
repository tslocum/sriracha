package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadBanForm(db *database.DB, r *http.Request, b *Ban) error {
	expire := FormString(r, "expire")
	if expire == "" {
		b.Expire = 0
	} else {
		timestamp, err := time.ParseInLocation("2006/01/02 15:04", expire, time.Local)
		if err != nil {
			return fmt.Errorf("failed to parse expire date and time (format: YYYY/MM/DD HH:MM)")
		}
		b.Expire = timestamp.Unix()
	}
	b.Reason = FormString(r, "reason")
	return nil
}

func (s *Server) serveBan(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_ban"
	data.Boards = db.AllBoards()

	deleteBanID := PathInt(r, "/sriracha/ban/delete/")
	if deleteBanID > 0 {
		if s.forbidden(w, data, "ban.delete") {
			return
		}
		b := db.BanByID(deleteBanID)
		if b == nil {
			data.ManageError("Invalid ban.")
			return
		}
		db.DeleteBan(b.ID)

		if strings.HasPrefix(b.IP, "r ") {
			s.reloadBans(db)
		}

		var changes string
		liftReason := FormString(r, "reason")
		if strings.TrimSpace(liftReason) != "" {
			changes = "Reason: " + liftReason
		}

		s.log(db, data.Account, nil, fmt.Sprintf("Lifted ban #%d", b.ID), changes)

		data.Redirect(w, r, "/sriracha/ban/")
		return
	}

	banID := PathInt(r, "/sriracha/ban/")
	if banID > 0 {
		data.Manage.Ban = db.BanByID(banID)

		if data.Manage.Ban != nil && r.Method == http.MethodPost {
			oldBan := *data.Manage.Ban
			err := s.loadBanForm(db, r, data.Manage.Ban)
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			shorter := data.Manage.Ban.Expire != 0 && (oldBan.Expire == 0 || data.Manage.Ban.Expire < oldBan.Expire)
			if shorter && s.forbidden(w, data, "ban.shorten") {
				return
			} else if !shorter && s.forbidden(w, data, "ban.lengthen") {
				return
			}

			err = data.Manage.Ban.Validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			db.UpdateBan(data.Manage.Ban)

			if strings.HasPrefix(data.Manage.Ban.IP, "r ") {
				s.reloadBans(db)
			}

			changes := printChanges(oldBan, *data.Manage.Ban)
			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/ban/%d", data.Manage.Ban.ID), changes)

			data.Redirect(w, r, "/sriracha/ban/")
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		f, _, err := r.FormFile("liftfile")
		if err == nil && f != nil {
			if s.forbidden(w, data, "banfile.delete") {
				return
			}
			buf, err := io.ReadAll(f)
			if err != nil {
				log.Fatal(err)
			}
			hash := s.hashBytes(buf, "")
			if db.FileBanned(hash) {
				db.LiftFileBan(hash)
				data.Info = "Lifted file ban."
				s.log(db, data.Account, nil, "Lifted file ban", "")
			} else {
				data.ManageError("File is not banned.")
			}
			return
		} else if s.forbidden(w, data, "ban.add") {
			return
		}

		b := &Ban{}
		err = s.loadBanForm(db, r, b)
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		ip := FormString(r, "ip")
		if strings.ContainsRune(ip, '*') {
			pattern := strings.ReplaceAll(strings.ReplaceAll(ip, ".", `\.`), "*", ".*")
			_, err := regexp.Compile(pattern)
			if err != nil {
				data.ManageError(fmt.Sprintf("failed to compile ban `%s` as regular expression: %s", pattern, err))
				return
			}
			b.IP = "r " + pattern
		} else if ip != "" {
			b.IP = s._hashIP(ip)
		}

		err = b.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		match := db.BanByIP(b.IP)
		if match != nil {
			data.ManageError("A ban for that IP address or range already exists.")
			return
		}

		db.AddBan(b)

		if strings.HasPrefix(b.IP, "r ") {
			s.reloadBans(db)
		}

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/ban/%d", b.ID), b.Info())

		data.Redirect(w, r, "/sriracha/ban/")
		return
	}

	data.Manage.Bans = db.AllBans(false)
	data.Page = PathInt(r, "/sriracha/ban/p")
	data.Pages = pageCount(len(data.Manage.Bans), entriesPerPage)
	data.Manage.Bans = pageSlice(data.Manage.Bans, data.Page, entriesPerPage)
}
