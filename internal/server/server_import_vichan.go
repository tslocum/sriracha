package server

import (
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
)

type vichanImport struct {
	db *sql.DB
}

func (v *vichanImport) Name() string {
	return "vichan"
}

func (v *vichanImport) tables() ([]string, error) {
	rows, err := v.db.Query("SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %s", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		err = rows.Scan(&table)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %s", err)
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list tables: %s", err)
	}
	return tables, nil
}

func (v *vichanImport) Matches() bool {
	tables, err := v.tables()
	if err != nil {
		return false
	}
	expected := []string{"antispam", "ban_appeals", "bans", "boards", "captchas", "cites", "flood", "ip_notes", "modlogs", "mods", "mutes", "news", "nntp_references", "noticeboard", "pages", "pms", "reports", "robot", "search_queries", "theme_settings"}
	for _, table := range expected {
		if !slices.Contains(tables, table) {
			return false
		}
	}
	return true
}

func (v *vichanImport) Tables() []string {
	var result []string
	tables, err := v.tables()
	if err != nil {
		log.Fatal(err)
	}
	for _, table := range tables {
		if strings.HasPrefix(table, "posts_") {
			result = append(result, table)
		}
	}
	return result
}

func (v *vichanImport) Posts(table string) []*Post {
	rows, err := v.db.Query("SELECT id, COALESCE(thread, 0), COALESCE(subject, ''), COALESCE(email, ''), COALESCE(name, ''), COALESCE(trip, ''), COALESCE(body, '') FROM " + table)
	if err != nil {
		log.Fatalf("failed to select posts: %s", err)
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		p := &Post{}
		err = rows.Scan(&p.ID, &p.Parent, &p.Subject, &p.Email, &p.Name, &p.Tripcode, &p.Message)
		if err != nil {
			log.Fatalf("failed to scan post: %s", err)
		}
		posts = append(posts, p)
	}
	if err = rows.Err(); err != nil {
		log.Fatalf("failed to select posts: %s", err)
	}
	return posts
}
