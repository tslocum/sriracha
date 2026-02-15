package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadPageForm(db *database.DB, r *http.Request, p *Page) error {
	p.Path = FormString(r, "path")
	p.Content = FormString(r, "content")

	p.Path = strings.TrimSuffix(p.Path, ".html")

	tpl, err := s.original.Clone()
	if err != nil {
		log.Fatal(err)
	}
	_, err = tpl.New("line").Parse(p.Content)
	if err != nil {
		return fmt.Errorf("failed to parse page content: %s", err)
	}
	return nil
}

func (s *Server) servePage(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}
	data.Template = "manage_page"
	data.Boards = db.AllBoards()
	data.Manage.Page = &Page{}

	deletePageID := PathInt(r, "/sriracha/page/delete/")
	if deletePageID > 0 {
		p := db.PageByID(deletePageID)
		if p == nil {
			data.ManageError("Invalid page.")
			return
		}
		db.DeletePage(p.ID)

		os.Remove(filepath.Join(s.config.Root, p.Path+".html"))

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted >>/page/%d", p.ID), "")

		http.Redirect(w, r, "/sriracha/page/", http.StatusFound)
		return
	}

	pageID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/page/"))
	if err == nil && pageID > 0 {
		p := db.PageByID(pageID)
		if p == nil {
			data.ManageError("Invalid page.")
			return
		} else if r.Method != http.MethodPost {
			return
		}
		data.Manage.Page = p

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

		if data.Manage.Page.Path != oldPath {
			match := db.PageByPath(data.Manage.Page.Path)
			if match != nil {
				data.ManageError("Page with that path already exists")
				return
			}
		}

		db.UpdatePage(data.Manage.Page)
		s.writePages(db, []*Page{data.Manage.Page})

		s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/page/%d", data.Manage.Page.ID), "")

		http.Redirect(w, r, "/sriracha/page/", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
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

		db.AddPage(p)
		s.writePages(db, []*Page{p})

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/page/%d", p.ID), "")

		http.Redirect(w, r, "/sriracha/page/", http.StatusFound)
		return
	}

	data.Manage.Pages = db.AllPages()
}
