package sriracha

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func (s *Server) serveKeyword(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}
	var err error
	data.Template = "manage_keyword"
	data.Boards = db.allBoards()

	keywordID := pathInt(r, "/sriracha/keyword/test/")
	if keywordID > 0 {
		data.Template = "manage_keyword_test"
		data.Manage.Keyword = db.keywordByID(keywordID)
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

	deleteKeywordID := pathInt(r, "/sriracha/keyword/delete/")
	if deleteKeywordID > 0 {
		k := db.keywordByID(deleteKeywordID)
		if k == nil {
			data.ManageError("Invalid keyword.")
			return
		}
		db.deleteKeyword(k.ID)

		db.log(data.Account, nil, fmt.Sprintf("Deleted >>/keyword/%d", k.ID), "")

		http.Redirect(w, r, "/sriracha/keyword/", http.StatusFound)
		return
	}

	keywordID, err = strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/keyword/"))
	if err == nil && keywordID > 0 {
		data.Manage.Keyword = db.keywordByID(keywordID)

		if data.Manage.Keyword != nil && r.Method == http.MethodPost {
			oldKeyword := *data.Manage.Keyword
			oldText := data.Manage.Keyword.Text
			data.Manage.Keyword.loadForm(db, r)

			err := data.Manage.Keyword.validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			if data.Manage.Keyword.Text != oldText {
				match := db.keywordByText(data.Manage.Keyword.Text)
				if match != nil {
					data.ManageError("Keyword text already exists")
					return
				}
			}

			db.updateKeyword(data.Manage.Keyword)

			changes := printChanges(oldKeyword, *data.Manage.Keyword)
			db.log(data.Account, nil, fmt.Sprintf("Updated >>/keyword/%d", data.Manage.Keyword.ID), changes)

			http.Redirect(w, r, "/sriracha/keyword/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		k := &Keyword{}
		k.loadForm(db, r)

		err := k.validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		match := db.keywordByText(k.Text)
		if match != nil {
			data.ManageError("Keyword text already exists")
			return
		}

		db.addKeyword(k)

		db.log(data.Account, nil, fmt.Sprintf("Added >>/keyword/%d", k.ID), "")

		http.Redirect(w, r, "/sriracha/keyword/", http.StatusFound)
		return
	}

	data.Manage.Keywords = db.allKeywords()
}
