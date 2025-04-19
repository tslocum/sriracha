package sriracha

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Server) serveBoard(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_board"

	boardID := pathInt(r, "/sriracha/board/rebuild/")
	if boardID > 0 {
		b := db.boardByID(boardID)
		if b != nil {
			s.rebuildBoard(db, b)

			data.Info = fmt.Sprintf("Rebuilt %s %s", b.Path(), b.Name)
		}
	}

	modBoard := pathString(r, "/sriracha/board/mod/")
	if modBoard != "" {
		var postID int
		split := strings.Split(modBoard, "/")
		if len(split) == 2 {
			boardID, _ = strconv.Atoi(split[0])
			postID, _ = strconv.Atoi(split[1])
		} else if len(split) == 1 {
			boardID, _ = strconv.Atoi(split[0])
		}

		b := db.boardByID(boardID)
		if b != nil {
			data.Template = "board_page"
			data.Board = b
			data.Boards = db.allBoards()
			data.ModMode = true
			if postID > 0 {
				data.Threads = [][]*Post{db.allPostsInThread(b, postID, true)}
				data.ReplyMode = postID
			} else {
				for _, thread := range db.allThreads(b, true) {
					data.Threads = append(data.Threads, db.allPostsInThread(b, thread.ID, true))
				}
			}
			return
		}

		data.ManageError("Invalid or deleted board or post")
		return
	}

	boardID = pathInt(r, "/sriracha/board/")
	if boardID > 0 {
		data.Manage.Board = db.boardByID(boardID)

		if data.Manage.Board != nil && r.Method == http.MethodPost {
			oldBoard := *data.Manage.Board

			oldDir := data.Manage.Board.Dir
			data.Manage.Board.loadForm(r)

			err := data.Manage.Board.validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			if data.Manage.Board.Dir != oldDir {
				_, err := os.Stat(filepath.Join(s.config.Root, data.Manage.Board.Dir))
				if err != nil {
					if !os.IsNotExist(err) {
						log.Fatal(err)
					}
				} else {
					data.ManageError("New directory already exists")
					return
				}
			}

			db.updateBoard(data.Manage.Board)

			if data.Manage.Board.Dir != oldDir {
				err := os.Rename(filepath.Join(s.config.Root, oldDir), filepath.Join(s.config.Root, data.Manage.Board.Dir))
				if err != nil {
					data.ManageError(fmt.Sprintf("Failed to rename board directory: %s", err))
					return
				}
			}

			s.rebuildBoard(db, data.Manage.Board)

			changes := printChanges(oldBoard, *data.Manage.Board)
			db.log(data.Account, nil, fmt.Sprintf("Updated >>/board/%d", data.Manage.Board.ID), changes)

			http.Redirect(w, r, "/sriracha/board/", http.StatusFound)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		b := &Board{}
		b.loadForm(r)

		err := b.validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		dirs := []string{"", "src", "thumb", "res"}
		for _, boardDir := range dirs {
			if b.Dir == "" && boardDir == "" {
				continue
			}
			boardPath := filepath.Join(s.config.Root, b.Dir, boardDir)
			err = os.Mkdir(boardPath, 0755)
			if err != nil {
				if os.IsExist(err) {
					data.ManageError(fmt.Sprintf("Board directory %s already exists.", boardPath))
				} else {
					data.ManageError(fmt.Sprintf("Failed to create board directory %s: %s", boardPath, err))
				}
				return
			}
		}

		db.addBoard(b)

		s.rebuildBoard(db, b)

		db.log(data.Account, nil, fmt.Sprintf("Added >>/board/%d", b.ID), "")

		http.Redirect(w, r, "/sriracha/board/", http.StatusFound)
		return
	}

	data.Manage.Board = &Board{
		Threads:     defaultBoardThreads,
		Replies:     defaultBoardReplies,
		MaxName:     defaultBoardMaxName,
		MaxEmail:    defaultBoardMaxEmail,
		MaxSubject:  defaultBoardMaxSubject,
		MaxMessage:  defaultBoardMaxMessage,
		WordBreak:   defaultBoardWordBreak,
		Truncate:    defaultBoardTruncate,
		MaxSize:     defaultBoardMaxSize,
		ThumbWidth:  defaultBoardThumbWidth,
		ThumbHeight: defaultBoardThumbHeight,
	}

	data.Manage.Boards = db.allBoards()
}
