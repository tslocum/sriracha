package database

import (
	"context"
	"log"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddPage(p *Page) {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO page VALUES (DEFAULT, $1, $2)",
		p.Path,
		p.Content,
	)
	if err != nil {
		log.Fatalf("failed to insert page: %s", err)
	}
	err = db.conn.QueryRow(context.Background(), "SELECT id FROM page WHERE path = $1", p.Path).Scan(&p.ID)
	if err != nil {
		log.Fatalf("failed to select id of added page: %s", err)
	} else if p.ID == 0 {
		log.Fatal("failed to select id of added page")
	}
}

func (db *DB) PageByID(id int) *Page {
	p := &Page{}
	err := scanPage(p, db.conn.QueryRow(context.Background(), "SELECT * FROM page WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select page: %s", err)
	}
	return p
}

func (db *DB) PageByPath(path string) *Page {
	p := &Page{}
	err := scanPage(p, db.conn.QueryRow(context.Background(), "SELECT * FROM page WHERE path = $1", path))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select page: %s", err)
	}
	return p
}

func (db *DB) AllPages() []*Page {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM page ORDER BY path ASC")
	if err != nil {
		log.Fatalf("failed to select all pages: %s", err)
	}
	var pages []*Page
	for rows.Next() {
		p := &Page{}
		err := scanPage(p, rows)
		if err != nil {
			log.Fatalf("failed to select all pages: %s", err)
		}
		pages = append(pages, p)
	}
	return pages
}

func (db *DB) UpdatePage(p *Page) {
	if p.ID <= 0 {
		log.Fatalf("invalid page ID %d", p.ID)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE page SET path = $1, content = $2 WHERE id = $3",
		p.Path,
		p.Content,
		p.ID,
	)
	if err != nil {
		log.Fatalf("failed to update page: %s", err)
	}
}

func (db *DB) DeletePage(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM page WHERE id = $1", id)
	if err != nil {
		log.Fatalf("failed to delete page: %s", err)
	}
}

func scanPage(p *Page, row pgx.Row) error {
	return row.Scan(
		&p.ID,
		&p.Path,
		&p.Content,
	)
}
