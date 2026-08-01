package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
)

type vichanImport struct {
	db *sql.DB
}

type vichanFileInfo struct {
	Name      string
	FullPath  string `json:"full_path"`
	Type      string
	FilePath  string `json:"file_path"`
	ThumbPath string `json:"thumb_path"`
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
	rows, err := v.db.Query("SELECT id, COALESCE(thread, 0), COALESCE(subject, ''), COALESCE(email, ''), COALESCE(name, ''), COALESCE(trip, ''), COALESCE(body, ''), COALESCE(files, ''), sticky, locked FROM " + table)
	if err != nil {
		log.Fatalf("failed to select posts: %s", err)
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		var (
			p = &Post{
				Moderated: ModeratedVisible,
			}
			fileInfo []byte
			stickied int
			locked   int
		)
		err = rows.Scan(&p.ID, &p.Parent, &p.Subject, &p.Email, &p.Name, &p.Tripcode, &p.Message, &fileInfo, &stickied, &locked)
		if err != nil {
			log.Fatalf("failed to scan post: %s", err)
		} else if len(fileInfo) > 0 {
			var allInfo []*vichanFileInfo
			err = json.Unmarshal(fileInfo, &allInfo)
			if err != nil {
				log.Fatalf("failed to scan files column: %s: %s", string(fileInfo), err)
			}
			for i, info := range allInfo {
				// store extra attachments, return as extra new posts duplicated from original and copied with file at tail
				if i != 0 {
					// store
					continue
				}
				lastSlash := strings.LastIndexByte(info.FilePath, '/')
				if lastSlash == -1 {
					log.Fatalf("failed to parse file %s: no slash found in file path", string(fileInfo))
				}
				p.File = info.FilePath[lastSlash+1:]
				if p.File == "" {
					log.Fatalf("failed to parse file %s: blank file")
				}
				lastSlash = strings.LastIndexByte(info.ThumbPath, '/')
				if lastSlash != -1 {
					p.Thumb = info.ThumbPath[lastSlash+1:]
				}
			}
		}
		p.Stickied = stickied == 1
		p.Locked = locked == 1
		posts = append(posts, p)
	}
	if err = rows.Err(); err != nil {
		log.Fatalf("failed to select posts: %s", err)
	}
	return posts
}
