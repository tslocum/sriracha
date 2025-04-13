package sriracha

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func (s *Server) serveKeyword(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	var err error
	data.Template = "manage_keyword"
	data.Boards, err = db.allBoards()
	if err != nil {
		log.Fatal(err)
	}

	keywordID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/imgboard/keyword/rest/"))
	if err == nil && keywordID > 0 {
		data.Template = "manage_keyword_test"
		data.Manage.Keyword, err = db.keywordByID(keywordID)
		if err != nil {
			log.Fatal(err)
		}
		if data.Manage.Keyword != nil && r.Method == http.MethodPost {
			rgxp, err := regexp.Compile(data.Manage.Keyword.Text)
			if err != nil {
				data.Error(fmt.Sprintf("Failed to compile regular expression: %s", err))
			}

			message := r.FormValue("message")
			match := rgxp.MatchString(message)
			if match {
				data.Info = "Result: MATCH FOUND"
			} else {
				data.Info = "Result: NO MATCH"
			}
		}
		return
	}

	keywordID, err = strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/imgboard/keyword/"))
	if err == nil && keywordID > 0 {
		data.Manage.Keyword, err = db.keywordByID(keywordID)
		if err != nil {
			log.Fatal(err)
		}

		if data.Manage.Keyword != nil && r.Method == http.MethodPost {
			oldText := data.Manage.Keyword.Text
			data.Manage.Keyword.loadForm(db, r)

			err := data.Manage.Keyword.validate()
			if err != nil {
				data.Error(err.Error())
				return
			}

			if data.Manage.Keyword.Text != oldText {
				match, err := db.keywordByText(data.Manage.Keyword.Text)
				if err != nil {
					log.Fatal(err)
				} else if match != nil {
					data.Error("Keyword text already exists")
					return
				}
			}

			err = db.updateKeyword(data.Manage.Keyword)
			if err != nil {
				data.Error(err.Error())
				return
			}

			err = db.log(data.Account, nil, fmt.Sprintf("Updated >>/keyword/%d", data.Manage.Keyword.ID))
			if err != nil {
				log.Fatal(err)
			}
			http.Redirect(w, r, "/imgboard/keyword/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		k := &Keyword{}
		k.loadForm(db, r)

		err := k.validate()
		if err != nil {
			data.Error(err.Error())
			return
		}

		match, err := db.keywordByText(k.Text)
		if err != nil {
			log.Fatal(err)
		} else if match != nil {
			data.Error("Keyword text already exists")
			return
		}

		err = db.addKeyword(k)
		if err != nil {
			log.Fatal(err)
			return
		}

		err = db.log(data.Account, nil, fmt.Sprintf("Added >>/keyword/%d", k.ID))
		if err != nil {
			log.Fatal(err)
		}
		http.Redirect(w, r, "/imgboard/keyword/", http.StatusFound)
		return
	}

	data.Manage.Keywords, err = db.allKeywords()
	if err != nil {
		log.Fatal(err)
	}
}
