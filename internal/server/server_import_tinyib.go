package server

import (
	"database/sql"
	"log"

	. "codeberg.org/tslocum/sriracha/model"
)

type tinyibImport struct {
	db *sql.DB
}

func (t *tinyibImport) Name() string {
	return "TinyIB"
}
func (t *tinyibImport) Matches() bool {
	const column = "file_size"
	var table string
	rows, err := t.db.Query("SELECT DISTINCT name FROM sqlite_master WHERE sql LIKE '%" + column + "%'")
	if err != nil {
		return false
	}
	for rows.Next() {
		err = rows.Scan(&table)
		if err != nil {
			return false
		}
	}
	if rows.Err() != nil {
		return false
	}
	return table != ""
}

func (t *tinyibImport) Tables() []string {
	return nil
}

func (t *tinyibImport) Posts(table string) []*Post {
	query := "SELECT id, parent, timestamp, bumped, name, tripcode, email, nameblock, subject, message, file, '' AS file_mime, file_hex, file_original, file_size, image_width, image_height, thumb, thumb_width, thumb_height, stickied, locked FROM " + table + " ORDER BY id ASC"

	// Query database for posts.
	var posts []*Post
	rows, err := t.db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		p := &Post{}
		var stickied, locked int
		err = rows.Scan(&p.ID,
			&p.Parent,
			&p.Timestamp,
			&p.Bumped,
			&p.Name,
			&p.Tripcode,
			&p.Email,
			&p.NameBlock,
			&p.Subject,
			&p.Message,
			&p.File,
			&p.FileMIME,
			&p.FileHash,
			&p.FileOriginal,
			&p.FileSize,
			&p.FileWidth,
			&p.FileHeight,
			&p.Thumb,
			&p.ThumbWidth,
			&p.ThumbHeight,
			&stickied,
			&locked)
		if err != nil {
			log.Fatal(err)
		}
		p.Moderated = ModeratedVisible
		p.Stickied = stickied == 1
		p.Locked = locked == 1

		posts = append(posts, p)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return posts
}
