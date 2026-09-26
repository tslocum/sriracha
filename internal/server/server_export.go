package server

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
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
		for _, p := range db.AllPostsInThread(FilterAny, thread[0]) {
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

func (s *Server) exportPosts(db serverDB, exportPath string, mini bool) error {
	boards := db.AllBoards()
	if len(boards) == 0 {
		return fmt.Errorf("no boards available to export")
	}
	var havePosts bool
	for _, b := range boards {
		if len(db.AllThreads(FilterAny, b)) > 0 {
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
	zipFile, err := os.OpenFile(exportPath, NewFileFlags, NewFilePermission)
	if err != nil {
		return fmt.Errorf("failed to open zip file %s: %s", exportPath, err)
	}
	defer zipFile.Close()

	z := zip.NewWriter(zipFile)

	date := time.Now().Format("20060102")
	for _, b := range boards {
		threads := db.AllThreads(FilterAny, b)
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
		boardName := fName
		fName += ".db"

		boardFile, err := s._exportBoardPosts(db, b, threads)
		if err != nil {
			return fmt.Errorf("failed to export board %s: %s", b.Path(), err)
		}
		zipBoardFile, err := z.Create(fName)
		if err != nil {
			return fmt.Errorf("failed to create file in zip archive: %s", err)
		}
		_, err = io.Copy(zipBoardFile, boardFile)
		if err != nil {
			return fmt.Errorf("failed to write zip archive: %s", err)
		}
		boardFile.Close()
		if !mini {
			var boardDir, srcDir, thumbDir bool
			for _, thread := range threads {
				for _, p := range db.AllPostsInThread(FilterAny, thread[0]) {
					if p.File != "" && !p.IsEmbed() {
						if !boardDir {
							_, err := z.Create(boardName + "/")
							if err != nil {
								return fmt.Errorf("failed to create directory in zip archive: %s", err)
							}
							boardDir = true
						}
						if !srcDir {
							_, err := z.Create(filepath.Join(boardName, "src") + "/")
							if err != nil {
								return fmt.Errorf("failed to create directory in zip archive: %s", err)
							}
							srcDir = true
						}
						srcPath := filepath.Join(s.config.Root, p.Board.Dir, "src", p.File)
						srcFile, err := os.Open(srcPath)
						if err != nil {
							return fmt.Errorf("failed to open file %s of post %d: %s", srcPath, p.ID, err)
						}
						srcZipFile, err := z.Create(filepath.Join(boardName, "src", p.File))
						if err != nil {
							return fmt.Errorf("failed to create file in zip archive: %s", err)
						}
						_, err = io.Copy(srcZipFile, srcFile)
						if err != nil {
							return fmt.Errorf("failed to write zip archive: %s", err)
						}
						srcFile.Close()
					}
					if p.Thumb != "" {
						if !boardDir {
							_, err := z.Create(boardName + "/")
							if err != nil {
								return fmt.Errorf("failed to create directory in zip archive: %s", err)
							}
							boardDir = true
						}
						if !thumbDir {
							_, err := z.Create(filepath.Join(boardName, "thumb") + "/")
							if err != nil {
								return fmt.Errorf("failed to create directory in zip archive: %s", err)
							}
							thumbDir = true
						}
						thumbPath := filepath.Join(s.config.Root, p.Board.Dir, "thumb", p.Thumb)
						thumbFile, err := os.Open(thumbPath)
						if err != nil {
							return fmt.Errorf("failed to open file %s of post %d: %s", thumbPath, p.ID, err)
						}
						thumbZipFile, err := z.Create(filepath.Join(boardName, "thumb", p.Thumb))
						if err != nil {
							return fmt.Errorf("failed to create file in zip archive: %s", err)
						}
						_, err = io.Copy(thumbZipFile, thumbFile)
						if err != nil {
							return fmt.Errorf("failed to write zip archive: %s", err)
						}
						thumbFile.Close()
					}
				}
			}
		}
	}

	err = z.Close()
	if err != nil {
		return fmt.Errorf("failed to write zip archive: %s", err)
	}

	if !mini {
		fmt.Printf("Exported post data and attachments to %s\n", exportPath)
		return nil
	}
	fmt.Printf("Exported post data to %s\n", exportPath)
	fmt.Printf("Warning: Attachment files are not included within the export. To import posts later, you will also need a copy of the src and thumb directories of each board.\n")
	return nil
}
