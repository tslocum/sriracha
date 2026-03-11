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

	categoryAndBoard := func(s string) (*Category, *Board) {
		split := strings.Split(s, "/")
		if len(split) != 2 {
			return nil, nil
		}
		categoryID, err := strconv.Atoi(split[0])
		if err != nil {
			return nil, nil
		}
		boardID, err := strconv.Atoi(split[1])
		if err != nil {
			return nil, nil
		}
		return db.CategoryByID(categoryID), db.BoardByID(boardID)
	}

	deleteBoardCategory := PathString(r, "/sriracha/category/board/delete/")
	if deleteBoardCategory != "" {
		if s.forbidden(w, data, "category.update") {
			return
		}
		category, board := categoryAndBoard(deleteBoardCategory)
		if category == nil {
			data.ManageError("Invalid category.")
			return
		} else if board == nil {
			data.ManageError("Invalid board.")
			return
		}

		for i, b := range category.Boards {
			if b.ID == board.ID {
				category.Boards = append(category.Boards[:i], category.Boards[i+1:]...)
				db.UpdateCategory(category)
				break
			}
		}

		http.Redirect(w, r, "/sriracha/category/", http.StatusFound)
		return
	}

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

	boardUp := PathString(r, "/sriracha/category/board/up/")
	if boardUp != "" {
		if s.forbidden(w, data, "category.update") {
			return
		}
		category, board := categoryAndBoard(boardUp)
		if category != nil && board != nil {
			for i, b := range category.Boards {
				if b.ID == board.ID && i > 0 {
					category.Boards[i], category.Boards[i-1] = category.Boards[i-1], category.Boards[i]
					db.UpdateCategory(category)
					break
				}
			}
		}
		http.Redirect(w, r, "/sriracha/category/", http.StatusFound)
		return
	}
	boardDown := PathString(r, "/sriracha/category/board/down/")
	if boardDown != "" {
		if s.forbidden(w, data, "category.update") {
			return
		}
		category, board := categoryAndBoard(boardDown)
		if category != nil && board != nil {
			for i, b := range category.Boards {
				if b.ID == board.ID && i < len(category.Boards)-1 {
					category.Boards[i], category.Boards[i+1] = category.Boards[i+1], category.Boards[i]
					db.UpdateCategory(category)
					break
				}
			}
		}
		http.Redirect(w, r, "/sriracha/category/", http.StatusFound)
		return
	}

	categoryUpID := PathInt(r, "/sriracha/category/up/")
	if categoryUpID != 0 {
		if s.forbidden(w, data, "category.update") {
			return
		}
		category := db.CategoryByID(categoryUpID)
		if category != nil {
			if category.Parent != nil {
				category.Parent.Categories = db.ChildCategories(category.Parent.ID)
				for i, c := range category.Parent.Categories {
					if c.ID == category.ID && i > 0 {
						category.Parent.Categories[i], category.Parent.Categories[i-1] = category.Parent.Categories[i-1], category.Parent.Categories[i]
						break
					}
				}
				for i, c := range category.Parent.Categories {
					c.Sort = i
					db.UpdateCategory(c)
				}
			} else {
				var rootCategories []*Category
				for _, c := range data.Categories {
					if c.Parent == nil {
						rootCategories = append(rootCategories, c)
					}
				}
				for i, c := range rootCategories {
					if c.ID == category.ID && i > 0 {
						rootCategories[i], rootCategories[i-1] = rootCategories[i-1], rootCategories[i]
						break
					}
				}
				for i, c := range rootCategories {
					c.Sort = i
					db.UpdateCategory(c)
				}
			}
		}
		http.Redirect(w, r, "/sriracha/category/", http.StatusFound)
		return
	}
	categoryDownID := PathInt(r, "/sriracha/category/down/")
	if categoryDownID != 0 {
		if s.forbidden(w, data, "category.update") {
			return
		}
		category := db.CategoryByID(categoryDownID)
		if category != nil {
			if category.Parent != nil {
				category.Parent.Categories = db.ChildCategories(category.Parent.ID)
				for i, c := range category.Parent.Categories {
					if c.ID == category.ID && i < len(category.Parent.Categories)-1 {
						category.Parent.Categories[i], category.Parent.Categories[i+1] = category.Parent.Categories[i+1], category.Parent.Categories[i]
						break
					}
				}
				for i, c := range category.Parent.Categories {
					c.Sort = i
					db.UpdateCategory(c)
				}
			} else {
				var rootCategories []*Category
				for _, c := range data.Categories {
					if c.Parent == nil {
						rootCategories = append(rootCategories, c)
					}
				}
				for i, c := range rootCategories {
					if c.ID == category.ID && i < len(rootCategories)-1 {
						rootCategories[i], rootCategories[i+1] = rootCategories[i+1], rootCategories[i]
						break
					}
				}
				for i, c := range rootCategories {
					c.Sort = i
					db.UpdateCategory(c)
				}
			}
		}
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
