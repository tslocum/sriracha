package server

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	_ "modernc.org/sqlite"
)

func (s *Server) _exportBoardPosts(db serverDB, b *Board, threads [][2]int) (*os.File, error) {
	f, err := os.CreateTemp("", "*.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %s", err)
	}

	export, err := sql.Open("sqlite", f.Name())
	if err != nil {
		log.Fatalf("failed to open temporary file %s: %s", f.Name(), err)
	}
	defer export.Close()

	_, err = export.Exec(`
CREATE TABLE post (
	id           INTEGER PRIMARY KEY,
	parent       INTEGER NOT NULL,
	timestamp    INTEGER NOT NULL,
	bumped       INTEGER NOT NULL,
	name         TEXT NOT NULL,
	tripcode     TEXT NOT NULL,
	email        TEXT NOT NULL,
	nameblock    TEXT NOT NULL,
	subject      TEXT NOT NULL,
	message      TEXT NOT NULL,
	file         TEXT NOT NULL,
	filemime     TEXT NOT NULL,
	filehash     TEXT NOT NULL,
	fileoriginal TEXT NOT NULL,
	filesize     INTEGER NOT NULL,
	filewidth    INTEGER NOT NULL,
	fileheight   INTEGER NOT NULL,
	thumb        TEXT NOT NULL,
	thumbwidth   INTEGER NOT NULL,
	thumbheight  INTEGER NOT NULL,
	stickied     INTEGER NOT NULL,
	locked       INTEGER NOT NULL
);`)
	if err != nil {
		return nil, fmt.Errorf("failed to create post schema: %s", err)
	}

	var hash string
	var stickied, locked int
	for _, thread := range threads {
		for _, p := range db.AllPostsInThread(thread[0], false) {
			if p.IsEmbed() {
				hash = p.FileHash
			} else {
				hash = ""
			}
			if p.Stickied {
				stickied = 1
			} else {
				stickied = 0
			}
			if p.Locked {
				locked = 1
			} else {
				locked = 0
			}
			_, err = export.Exec("INSERT INTO post VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
				p.ID,
				p.Parent,
				p.Timestamp,
				p.Bumped,
				p.Name,
				p.Tripcode,
				p.Email,
				p.NameBlock,
				p.Subject,
				p.Message,
				p.File,
				p.FileMIME,
				hash,
				p.FileOriginal,
				p.FileSize,
				p.FileWidth,
				p.FileHeight,
				p.Thumb,
				p.ThumbWidth,
				p.ThumbHeight,
				stickied,
				locked)
			if err != nil {
				return nil, fmt.Errorf("failed to export post: %s", err)
			}
		}
	}
	return f, nil
}

func (s *Server) exportPosts(db serverDB, exportPath string) error {
	boards := db.AllBoards()
	if len(boards) == 0 {
		return fmt.Errorf("no boards available to export")
	}
	var havePosts bool
	for _, b := range boards {
		if len(db.AllThreads(b, false)) > 0 {
			havePosts = true
			continue
		}
	}
	if !havePosts {
		return fmt.Errorf("no posts available to export")
	}

	_, err := os.Stat(exportPath)
	if !os.IsNotExist(err) {
		return fmt.Errorf("file %s already exists", exportPath)
	}
	zipFile, err := os.OpenFile(exportPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		return fmt.Errorf("failed to open zip file %s: %s", exportPath, err)
	}
	defer zipFile.Close()

	zip := zip.NewWriter(zipFile)

	date := time.Now().Format("20060102")
	for _, b := range boards {
		threads := db.AllThreads(b, false)
		if len(threads) == 0 {
			continue
		}
		fmt.Printf("Exporting %s...\n", b.Path())

		fName := date
		if b.Dir == "" {
			fName += "_root"
		} else {
			fName += "_" + strings.ToLower(b.Dir)
		}
		if b.Description != "" {
			fName += "_" + strings.ReplaceAll(strings.ToLower(b.Name), " ", "_")
		}
		fName += ".serverDB"

		boardFile, err := s._exportBoardPosts(db, b, threads)
		if err != nil {
			return fmt.Errorf("failed to export board %s: %s", b.Path(), err)
		}
		zipBoardFile, err := zip.Create(fName)
		if err != nil {
			return fmt.Errorf("failed to create file in zip archive: %s", err)
		}
		_, err = io.Copy(zipBoardFile, boardFile)
		if err != nil {
			return fmt.Errorf("failed to write zip archive: %s", err)
		}
		boardFile.Close()
	}

	err = zip.Close()
	if err != nil {
		return fmt.Errorf("failed to write zip archive: %s", err)
	}

	fmt.Printf("Exported post data to %s\n", exportPath)
	fmt.Printf("Warning: Attachment files are not included within the export. To import posts later, you will also need a copy of the src and thumb directories of each board.\n")
	return nil
}
