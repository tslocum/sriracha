package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadNewsForm(db *database.DB, r *http.Request, n *News, a *Account) {
	n.Timestamp = FormInt64(r, "timestamp")
	if n.Account != nil && n.Account.ID == a.ID {
		n.Share = FormBool(r, "share")
	}
	n.Name = FormString(r, "name")
	n.Subject = FormString(r, "subject")
	n.Message = FormString(r, "message")
}

func (s *Server) serveNews(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	var err error
	data.Template = "manage_news"
	data.Boards = db.AllBoards()

	deleteNewsID := PathInt(r, "/sriracha/news/delete/")
	if deleteNewsID > 0 {
		news := db.NewsByID(deleteNewsID)
		if news == nil {
			data.ManageError("Invalid news item.")
			return
		} else if !news.MayDelete(data.Account) {
			data.ManageError("Access denied.")
			return
		}

		db.DeleteNews(deleteNewsID)
		os.Remove(filepath.Join(s.config.Root, fmt.Sprintf("news-%d.html", news.ID)))

		s.writeNewsIndexes(db)

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted >>/news/%d", deleteNewsID), "")

		http.Redirect(w, r, "/sriracha/news/", http.StatusFound)
		return
	}

	newsID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/news/"))
	if err == nil && newsID > 0 {
		data.Manage.News = db.NewsByID(newsID)

		if data.Manage.News != nil && r.Method == http.MethodPost {
			if !data.Manage.News.MayUpdate(data.Account) {
				data.ManageError("Access denied.")
				return
			}
			oldNews := *data.Manage.News
			s.loadNewsForm(db, r, data.Manage.News, data.Account)

			err := data.Manage.News.Validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			db.UpdateNews(data.Manage.News)

			if data.Manage.News.Timestamp == 0 || data.Manage.News.Timestamp > time.Now().Unix() {
				os.Remove(filepath.Join(s.config.Root, fmt.Sprintf("news-%d.html", data.Manage.News.ID)))
				s.writeNewsIndexes(db)
			} else {
				s.rebuildNewsItem(db, data.Manage.News)
			}

			changes := printChanges(oldNews, *data.Manage.News)
			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/news/%d", data.Manage.News.ID), changes)

			http.Redirect(w, r, "/sriracha/news/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		n := &News{}
		n.Account = data.Account
		s.loadNewsForm(db, r, n, data.Account)

		err := n.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		db.AddNews(n)
		if n.Timestamp != 0 && n.Timestamp <= time.Now().Unix() {
			s.rebuildNewsItem(db, n)
		}

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/news/%d", n.ID), "")

		http.Redirect(w, r, "/sriracha/news/", http.StatusFound)
		return
	}

	data.Manage.AllNews = db.AllNews(false)
}
