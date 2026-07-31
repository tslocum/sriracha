package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadPageForm(db serverDB, r *http.Request, p *Page) error {
	p.Path = FormString(r, "path")
	p.Content = FormString(r, "content")

	p.Path = strings.TrimSuffix(p.Path, ".html")

	return s.writePage(db, nil, p, io.Discard)
}

func (s *Server) servePage(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_page"
	data.Boards = db.AllBoards()
	data.Manage.Page = &Page{}

	if PathString(r, "/sriracha/page/rebuild/") != "" {
		if s.forbidden(w, data, "page.update") {
			return
		}
		var pages []*Page
		rebuildPageID := PathInt(r, "/sriracha/page/rebuild/")
		if rebuildPageID > 0 {
			p := db.PageByID(rebuildPageID)
			if p == nil {
				data.ManageError("Invalid page.")
				return
			}
			pages = append(pages, p)
			data.Info = Get(nil, data.Account, "Rebuilt %s.", p.Path)
		} else {
			pages = db.AllPages()
			data.Info = Get(nil, data.Account, "Rebuilt all pages.")
		}
		wg := &sync.WaitGroup{}
		s.writePages(db, wg, pages)
		wg.Wait()
	}

	deletePageID := PathInt(r, "/sriracha/page/delete/")
	if deletePageID > 0 {
		if s.forbidden(w, data, "page.delete") {
			return
		}
		p := db.PageByID(deletePageID)
		if p == nil {
			data.ManageError("Invalid page.")
			return
		}
		db.DeletePage(p.ID)

		os.Remove(filepath.Join(s.config.Root, p.Path+".html"))

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted page #%d", p.ID), "")

		data.Redirect(w, r, "/sriracha/page/")
		return
	}

	pageID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/page/"))
	if err == nil && pageID > 0 {
		if s.forbidden(w, data, "page.update") {
			return
		}
		p := db.PageByID(pageID)
		if p == nil {
			data.ManageError("Invalid page.")
			return
		}
		data.Manage.Page = p

		if r.Method != http.MethodPost {
			return
		}
		oldPath := data.Manage.Page.Path
		err = s.loadPageForm(db, r, data.Manage.Page)
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		err := data.Manage.Page.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		if FormString(r, "preview") != "" {
			buf := &bytes.Buffer{}
			err = s.writePage(db, nil, data.Manage.Page, buf)
			if err != nil {
				data.ManageError(err.Error())
				return
			}
			data.Template = ""
			io.Copy(w, buf)
			return
		}

		if data.Manage.Page.Path != oldPath {
			match := db.PageByPath(data.Manage.Page.Path)
			if match != nil {
				data.ManageError("Page with that path already exists")
				return
			}

			os.Remove(filepath.Join(s.config.Root, oldPath+".html"))
		}

		db.UpdatePage(data.Manage.Page)
		wg := &sync.WaitGroup{}
		s.writePages(db, wg, []*Page{data.Manage.Page})
		wg.Wait()

		s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/page/%d", data.Manage.Page.ID), "")

		data.Redirect(w, r, "/sriracha/page/")
		return
	}

	if r.Method == http.MethodPost {
		if s.forbidden(w, data, "page.add") {
			return
		}
		p := &Page{}
		err = s.loadPageForm(db, r, p)
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		err := p.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		match := db.PageByPath(p.Path)
		if match != nil {
			data.ManageError("Page with that path already exists")
			return
		}

		_, err = os.Stat(filepath.Join(s.config.Root, p.Path+".html"))
		if !os.IsNotExist(err) {
			data.ManageError("File already exists at that path")
			return
		}

		if FormString(r, "preview") != "" {
			buf := &bytes.Buffer{}
			err = s.writePage(db, nil, p, buf)
			if err != nil {
				data.ManageError(err.Error())
				return
			}
			data.Template = ""
			io.Copy(w, buf)
			return
		}

		db.AddPage(p)
		wg := &sync.WaitGroup{}
		s.writePages(db, wg, []*Page{p})
		wg.Wait()

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/page/%d", p.ID), "")

		data.Redirect(w, r, "/sriracha/page/")
		return
	}

	data.Manage.Pages = db.AllPages()
	data.Page = PathInt(r, "/sriracha/page/p")
	data.Pages = pageCount(len(data.Manage.Pages), entriesPerPage)
	data.Manage.Pages = pageSlice(data.Manage.Pages, data.Page, entriesPerPage)
}
