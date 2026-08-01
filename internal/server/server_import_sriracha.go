package server

import (
	"database/sql"
	"log"

	. "codeberg.org/tslocum/sriracha/model"
)

type srirachaImport struct {
	db *sql.DB
}

func (s *srirachaImport) Name() string {
	return "Sriracha"
}

func (s *srirachaImport) Matches() bool {
	const column = "filesize"
	var table string
	rows, err := s.db.Query("SELECT DISTINCT name FROM sqlite_master WHERE sql LIKE '%" + column + "%'")
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

func (s *srirachaImport) Tables() []string {
	return nil
}

func (s *srirachaImport) Posts(table string) []*Post {
	query := "SELECT id, parent, timestamp, bumped, name, tripcode, email, nameblock, subject, message, file, filemime, filehash, fileoriginal, filesize, filewidth, fileheight, thumb, thumbwidth, thumbheight, stickied, locked FROM " + table + " ORDER BY id ASC"

	// Query database for posts.
	var posts []*Post
	rows, err := s.db.Query(query)
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
