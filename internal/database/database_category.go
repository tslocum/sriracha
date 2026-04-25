package database

import (
	"context"
	"fmt"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddCategory(c *Category) {
	var parent *int
	if c.Parent != nil {
		parent = &c.Parent.ID
	}
	err := db.conn.QueryRow(context.Background(), "INSERT INTO category VALUES (DEFAULT, $1, $2, $3, $4) RETURNING id",
		parent,
		c.Sort,
		c.Name,
		c.Description,
	).Scan(&c.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert category: %w", err))
	}
	db.fetchCategoryData(c, parent)
}

func (db *DB) fetchCategoryData(c *Category, parent *int) {
	c.Parent = nil
	if parent != nil && *parent != 0 {
		c.Parent = db.CategoryByID(*parent)
	}

	c.Boards = nil
	rows, err := db.conn.Query(context.Background(), "SELECT board FROM category_board WHERE category = $1 ORDER BY sort ASC", c.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to select category boards: %w", err))
	}
	var ids []int
	for rows.Next() {
		var id int
		err := rows.Scan(&id)
		if err != nil {
			dbErr(fmt.Errorf("failed to select category boards: %w", err))
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select category boards: %w", rows.Err()))
	}
	for _, id := range ids {
		b := db.BoardByID(id)
		c.Boards = append(c.Boards, b)
	}
}

func (db *DB) updateCategoryBoards(c *Category) {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM category_board WHERE category = $1", c.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update category boards: %w", err))
	}
	for i, b := range c.Boards {
		_, err = db.conn.Exec(context.Background(), "INSERT INTO category_board VALUES ($1, $2, $3)", c.ID, b.ID, i)
		if err != nil {
			dbErr(fmt.Errorf("failed to update category boards: %w", err))
		}
	}
}

func (db *DB) CategoryByID(id int) *Category {
	c := &Category{}
	err, parent := scanCategory(c, db.conn.QueryRow(context.Background(), "SELECT * FROM category WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select category: %w", err))
	}
	db.fetchCategoryData(c, parent)
	return c
}

func (db *DB) ChildCategories(id int) []*Category {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM category WHERE parent = $1 ORDER BY parent ASC NULLS FIRST, sort ASC", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to select all categories: %w", err))
	}
	var categories []*Category
	var parents []*int
	for rows.Next() {
		c := &Category{}
		err, parent := scanCategory(c, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select child categories: %w", err))
		}
		categories = append(categories, c)
		parents = append(parents, parent)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all categories: %w", rows.Err()))
	}
	for i, c := range categories {
		db.fetchCategoryData(c, parents[i])
		c.Categories = db.ChildCategories(c.ID)
	}
	return categories
}

func (db *DB) AllCategories() []*Category {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM category ORDER BY parent ASC NULLS FIRST, sort ASC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select all categories: %w", err))
	}
	var categories []*Category
	var parents []*int
	for rows.Next() {
		c := &Category{}
		err, parent := scanCategory(c, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select all categories: %w", err))
		}
		categories = append(categories, c)
		parents = append(parents, parent)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all categories: %w", rows.Err()))
	}
	for i, c := range categories {
		db.fetchCategoryData(c, parents[i])
		c.Categories = db.ChildCategories(c.ID)
	}
	return categories
}

func (db *DB) UpdateCategory(c *Category) {
	if c.ID <= 0 {
		dbErr(fmt.Errorf("invalid category ID %d", c.ID))
	}
	var parent *int
	if c.Parent != nil {
		parent = &c.Parent.ID
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE category SET parent = $1, sort = $2, name = $3, description = $4 WHERE id = $5",
		parent,
		c.Sort,
		c.Name,
		c.Description,
		c.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update category: %w", err))
	}
	db.updateCategoryBoards(c)
}

func (db *DB) DeleteCategory(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM category WHERE id = $1", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete category: %w", err))
	}
}

func scanCategory(c *Category, row pgx.Row) (error, *int) {
	var parent *int
	err := row.Scan(
		&c.ID,
		&parent,
		&c.Sort,
		&c.Name,
		&c.Description,
	)
	return err, parent
}
