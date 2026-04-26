package database

import (
	"context"
	"fmt"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddPage(p *Page) {
	err := db.conn.QueryRow(context.Background(), "INSERT INTO page VALUES (DEFAULT, $1, $2) RETURNING id",
		p.Path,
		p.Content,
	).Scan(&p.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert page: %w", err))
	}
}

func (db *DB) PageByID(id int) *Page {
	p := &Page{}
	err := scanPage(p, db.conn.QueryRow(context.Background(), "SELECT * FROM page WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select page: %w", err))
	}
	return p
}

func (db *DB) PageByPath(path string) *Page {
	p := &Page{}
	err := scanPage(p, db.conn.QueryRow(context.Background(), "SELECT * FROM page WHERE path = $1", path))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select page: %w", err))
	}
	return p
}

func (db *DB) AllPages() []*Page {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM page ORDER BY path ASC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select all pages: %w", err))
	}
	var pages []*Page
	for rows.Next() {
		p := &Page{}
		err := scanPage(p, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select all pages: %w", err))
		}
		pages = append(pages, p)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all pages: %w", rows.Err()))
	}
	return pages
}

func (db *DB) UpdatePage(p *Page) {
	if p.ID <= 0 {
		dbErr(fmt.Errorf("invalid page ID %d", p.ID))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE page SET path = $1, content = $2 WHERE id = $3",
		p.Path,
		p.Content,
		p.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update page: %w", err))
	}
}

func (db *DB) DeletePage(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM page WHERE id = $1", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete page: %w", err))
	}
}

func scanPage(p *Page, row pgx.Row) error {
	return row.Scan(
		&p.ID,
		&p.Path,
		&p.Content,
	)
}
