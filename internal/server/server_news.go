package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadNewsForm(db serverDB, r *http.Request, n *News, a *Account) error {
	ts := FormString(r, "timestamp")
	if ts == "" {
		n.Timestamp = 0
	} else {
		timestamp, err := time.ParseInLocation("2006/01/02 15:04", strings.ReplaceAll(ts, "-", "/"), time.Local)
		if err != nil {
			return fmt.Errorf("failed to parse publish date and time (format: YYYY/MM/DD HH:MM)")
		}
		n.Timestamp = timestamp.Unix()
	}
	if n.Account != nil && n.Account.ID == a.ID {
		n.Share = FormBool(r, "share")
	}
	n.Name = FormString(r, "name")
	n.Subject = FormString(r, "subject")
	n.Message = FormString(r, "message")
	return nil
}

func (s *Server) serveNews(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	if s.opt.News == NewsDisable {
		data.ManageError("Site news is disabled.")
		return
	}

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
		s.pageTimingLock.Lock()
		delete(s.pageTimings, fmt.Sprintf("/news-%d.html", news.ID))
		s.pageTimingLock.Unlock()

		wg := &sync.WaitGroup{}
		delta := &atomic.Uint32{}
		db.SoftCommit()
		s.writeNewsIndexes(db, wg, delta)
		s.writeSiteIndex(wg, delta)
		wg.Wait()

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted news #%d", deleteNewsID), "")

		data.Redirect(w, r, "/sriracha/news/")
		return
	}

	newsID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/news/"))
	if err == nil && newsID > 0 {
		data.Manage.News = db.NewsByID(newsID)
		if data.Manage.News == nil {
			data.ManageError("Invalid news item.")
			return
		}

		if data.Manage.News != nil && r.Method == http.MethodPost {
			if !data.Manage.News.MayUpdate(data.Account) {
				data.ManageError("Access denied.")
				return
			}
			oldNews := *data.Manage.News
			err = s.loadNewsForm(db, r, data.Manage.News, data.Account)
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			err := data.Manage.News.Validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			db.UpdateNews(data.Manage.News)
			changes := printChanges(oldNews, *data.Manage.News)

			wg := &sync.WaitGroup{}
			delta := &atomic.Uint32{}
			db.SoftCommit()
			if data.Manage.News.Timestamp == 0 || data.Manage.News.Timestamp > time.Now().Unix() {
				os.Remove(filepath.Join(s.config.Root, fmt.Sprintf("news-%d.html", data.Manage.News.ID)))
				s.pageTimingLock.Lock()
				delete(s.pageTimings, fmt.Sprintf("/news-%d.html", data.Manage.News.ID))
				s.pageTimingLock.Unlock()
				s.writeNewsIndexes(db, wg, delta)
			} else {
				s.rebuildNewsEntry(db, wg, delta, data.Manage.News)
			}
			s.writeSiteIndex(wg, delta)
			wg.Wait()

			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/news/%d", data.Manage.News.ID), changes)

			data.Redirect(w, r, "/sriracha/news/")
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		n := &News{}
		n.Account = data.Account
		err = s.loadNewsForm(db, r, n, data.Account)
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		err := n.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		db.AddNews(n)
		db.SoftCommit()
		if n.Timestamp != 0 && n.Timestamp <= time.Now().Unix() {
			wg := &sync.WaitGroup{}
			delta := &atomic.Uint32{}
			s.rebuildNewsEntry(db, wg, delta, n)
			s.writeSiteIndex(wg, delta)
			wg.Wait()
		}

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/news/%d", n.ID), "")

		data.Redirect(w, r, "/sriracha/news/")
		return
	}

	data.Manage.AllNews = db.AllNews(false)
	data.Page = PathInt(r, "/sriracha/news/p")
	data.Pages = pageCount(len(data.Manage.AllNews), entriesPerPage)
	data.Manage.AllNews = pageSlice(data.Manage.AllNews, data.Page, entriesPerPage)
}
