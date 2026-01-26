package server

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadKeywordForm(db *database.DB, r *http.Request, k *Keyword) {
	k.Text = FormString(r, "text")
	k.Action = FormString(r, "action")
	k.Boards = nil
	boards := r.Form["boards"]
	for _, board := range boards {
		boardID, err := strconv.Atoi(board)
		if err != nil || boardID <= 0 {
			continue
		}
		b := db.BoardByID(boardID)
		if b == nil {
			continue
		}
		k.Boards = append(k.Boards, b)
	}
}

func (s *Server) serveKeyword(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}
	var err error
	data.Template = "manage_keyword"
	data.Boards = db.AllBoards()

	keywordID := PathInt(r, "/sriracha/keyword/test/")
	if keywordID > 0 {
		data.Template = "manage_keyword_test"
		data.Manage.Keyword = db.KeywordByID(keywordID)
		if data.Manage.Keyword != nil && r.Method == http.MethodPost {
			rgxp, err := regexp.Compile(data.Manage.Keyword.Text)
			if err != nil {
				data.ManageError(fmt.Sprintf("Failed to compile regular expression: %s", err))
			}

			message := r.FormValue("message")
			data.Extra = message

			match := rgxp.MatchString(message)
			matchLabel := "NO MATCH"
			if match {
				matchLabel = "MATCH FOUND"
			}
			data.Message = template.HTML(fmt.Sprintf(`Result: <b>%s</b>`, matchLabel))
		}
		return
	}

	deleteKeywordID := PathInt(r, "/sriracha/keyword/delete/")
	if deleteKeywordID > 0 {
		k := db.KeywordByID(deleteKeywordID)
		if k == nil {
			data.ManageError("Invalid keyword.")
			return
		}
		db.DeleteKeyword(k.ID)

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted >>/keyword/%d", k.ID), "")

		http.Redirect(w, r, "/sriracha/keyword/", http.StatusFound)
		return
	}

	keywordID, err = strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/keyword/"))
	if err == nil && keywordID > 0 {
		data.Manage.Keyword = db.KeywordByID(keywordID)

		if data.Manage.Keyword != nil && r.Method == http.MethodPost {
			oldKeyword := *data.Manage.Keyword
			oldText := data.Manage.Keyword.Text
			s.loadKeywordForm(db, r, data.Manage.Keyword)

			err := data.Manage.Keyword.Validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			if data.Manage.Keyword.Text != oldText {
				match := db.KeywordByText(data.Manage.Keyword.Text)
				if match != nil {
					data.ManageError("Keyword text already exists")
					return
				}
			}

			db.UpdateKeyword(data.Manage.Keyword)

			changes := printChanges(oldKeyword, *data.Manage.Keyword)
			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/keyword/%d", data.Manage.Keyword.ID), changes)

			http.Redirect(w, r, "/sriracha/keyword/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		k := &Keyword{}
		s.loadKeywordForm(db, r, k)

		err := k.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		match := db.KeywordByText(k.Text)
		if match != nil {
			data.ManageError("Keyword text already exists")
			return
		}

		db.AddKeyword(k)

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/keyword/%d", k.ID), "")

		http.Redirect(w, r, "/sriracha/keyword/", http.StatusFound)
		return
	}

	data.Manage.Keywords = db.AllKeywords()
}
