package sriracha

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *Server) serveNews(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	var err error
	data.Template = "manage_news"
	data.Boards = db.AllBoards()

	deleteNewsID := pathInt(r, "/sriracha/news/delete/")
	if deleteNewsID > 0 {
		news := db.newsByID(deleteNewsID)
		if news == nil {
			data.ManageError("Invalid news item.")
			return
		} else if !news.MayDelete(data.Account) {
			data.ManageError("Access denied.")
			return
		}

		db.deleteNews(deleteNewsID)
		os.Remove(filepath.Join(s.config.Root, fmt.Sprintf("news-%d.html", news.ID)))

		s.writeNewsIndexes(db)

		db.log(data.Account, nil, fmt.Sprintf("Deleted >>/news/%d", deleteNewsID), "")

		http.Redirect(w, r, "/sriracha/news/", http.StatusFound)
		return
	}

	newsID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/news/"))
	if err == nil && newsID > 0 {
		data.Manage.News = db.newsByID(newsID)

		if data.Manage.News != nil && r.Method == http.MethodPost {
			if !data.Manage.News.MayUpdate(data.Account) {
				data.ManageError("Access denied.")
				return
			}
			oldNews := *data.Manage.News
			data.Manage.News.loadForm(db, r, data.Account)

			err := data.Manage.News.validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			db.updateNews(data.Manage.News)

			if data.Manage.News.Timestamp == 0 || data.Manage.News.Timestamp > time.Now().Unix() {
				os.Remove(filepath.Join(s.config.Root, fmt.Sprintf("news-%d.html", data.Manage.News.ID)))
				s.writeNewsIndexes(db)
			} else {
				s.rebuildNewsItem(db, data.Manage.News)
			}

			changes := printChanges(oldNews, *data.Manage.News)
			db.log(data.Account, nil, fmt.Sprintf("Updated >>/news/%d", data.Manage.News.ID), changes)

			http.Redirect(w, r, "/sriracha/news/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		n := &News{}
		n.Account = data.Account
		n.loadForm(db, r, data.Account)

		err := n.validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		db.addNews(n)
		if n.Timestamp != 0 && n.Timestamp <= time.Now().Unix() {
			s.rebuildNewsItem(db, n)
		}

		db.log(data.Account, nil, fmt.Sprintf("Added >>/news/%d", n.ID), "")

		http.Redirect(w, r, "/sriracha/news/", http.StatusFound)
		return
	}

	data.Manage.AllNews = db.allNews(false)
}
