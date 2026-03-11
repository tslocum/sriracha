package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadCategoryForm(db *database.DB, r *http.Request, k *Category) {
	parent := FormInt(r, "parent")
	if parent == 0 {
		k.Parent = nil
	} else {
		k.Parent = db.CategoryByID(parent)
	}
	k.Name = FormString(r, "name")
	k.Description = FormString(r, "description")
}

func (s *Server) serveCategory(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_category"
	data.Boards = db.AllBoards()
	data.Manage.Category = &Category{}
	data.Categories = db.AllCategories()

	deleteCategoryID := PathInt(r, "/sriracha/category/delete/")
	if deleteCategoryID > 0 {
		if s.forbidden(w, data, "category.delete") {
			return
		}
		c := db.CategoryByID(deleteCategoryID)
		if c == nil {
			data.ManageError("Invalid category.")
			return
		}
		db.DeleteCategory(c.ID)

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted category #%d", c.ID), "")

		http.Redirect(w, r, "/sriracha/category/", http.StatusFound)
		return
	}

	boardCategoryID := PathInt(r, "/sriracha/category/board/")
	if boardCategoryID > 0 {
		if s.forbidden(w, data, "category.update") {
			return
		}
		data.Manage.Category = db.CategoryByID(boardCategoryID)
		data.Extra = "board"

		if data.Manage.Category != nil && r.Method == http.MethodPost {
			boardID := FormInt(r, "board")
			board := db.BoardByID(boardID)
			if board == nil {
				data.ManageError("Invalid board.")
				return
			}

			if !data.Manage.Category.HasBoard(board.ID) {
				data.Manage.Category.Boards = append(data.Manage.Category.Boards, board)
				db.UpdateCategory(data.Manage.Category)
			}

			http.Redirect(w, r, "/sriracha/category/", http.StatusFound)
			return
		}
		return
	}

	categoryID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/category/"))
	if err == nil && categoryID > 0 {
		if s.forbidden(w, data, "category.update") {
			return
		}
		data.Manage.Category = db.CategoryByID(categoryID)

		if data.Manage.Category != nil && r.Method == http.MethodPost {
			oldCategory := *data.Manage.Category
			s.loadCategoryForm(db, r, data.Manage.Category)

			db.UpdateCategory(data.Manage.Category)

			changes := printChanges(oldCategory, *data.Manage.Category)
			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/category/%d", data.Manage.Category.ID), changes)

			http.Redirect(w, r, "/sriracha/category/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		if s.forbidden(w, data, "category.add") {
			return
		}
		c := &Category{}
		s.loadCategoryForm(db, r, c)

		var cSort int
		if c.Parent != nil {
			for _, cat := range c.Categories {
				if cat.Sort > cSort {
					cSort = cat.Sort
				}
			}
		} else {
			for _, cat := range data.Categories {
				if cat.Parent == nil && cat.Sort > cSort {
					cSort = cat.Sort
				}
			}
		}
		c.Sort = cSort + 1

		db.AddCategory(c)

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/category/%d", c.ID), "")

		http.Redirect(w, r, "/sriracha/category/", http.StatusFound)
		return
	}
}
