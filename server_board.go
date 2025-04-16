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

	boardID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/board/rebuild/"))
	if err == nil && boardID > 0 {
		b, err := db.boardByID(boardID)
		if err != nil {
			log.Fatal(err)
		} else if b != nil {
			s.writeBoard(b)

			data.Info = fmt.Sprintf("Rebuilt /%s/ %s", b.Dir, b.Name)
		}
	}

	boardID, err = strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/board/"))
	if err == nil && boardID > 0 {
		data.Manage.Board, err = db.boardByID(boardID)
		if err != nil {
			log.Fatal(err)
		}

		if data.Manage.Board != nil && r.Method == http.MethodPost {
			oldDir := data.Manage.Board.Dir
			data.Manage.Board.loadForm(r)

			err := data.Manage.Board.validate()
			if err != nil {
				data.Error(err.Error())
				return
			}

			if data.Manage.Board.Dir != oldDir {
				_, err := os.Stat(filepath.Join(s.config.Root, data.Manage.Board.Dir))
				if err != nil {
					if !os.IsNotExist(err) {
						log.Fatal(err)
					}
				} else {
					data.Error("New directory already exists")
					return
				}
			}

			err = db.updateBoard(data.Manage.Board)
			if err != nil {
				data.Error(err.Error())
				return
			}

			if data.Manage.Board.Dir != oldDir {
				err := os.Rename(filepath.Join(s.config.Root, oldDir), filepath.Join(s.config.Root, data.Manage.Board.Dir))
				if err != nil {
					data.Error(fmt.Sprintf("Failed to rename board directory: %s", err))
					return
				}
			}

			s.writeBoard(data.Manage.Board)

			err = db.log(data.Account, nil, fmt.Sprintf("Updated >>/board/%d", data.Manage.Board.ID))
			if err != nil {
				log.Fatal(err)
			}
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
			data.Error(err.Error())
			return
		}

		err = os.Mkdir(filepath.Join(s.config.Root, b.Dir), 0755)
		if err != nil {
			if os.IsExist(err) {
				data.Error(fmt.Sprintf("Board directory %s already exists.", b.Dir))
			} else {
				data.Error(fmt.Sprintf("Failed to create board directory %s: %s", b.Dir, err))
			}
			return
		}

		err = db.addBoard(b)
		if err != nil {
			data.Error(err.Error())
			return
		}

		s.writeBoard(b)

		err = db.log(data.Account, nil, fmt.Sprintf("Added >>/board/%d", b.ID))
		if err != nil {
			log.Fatal(err)
		}
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

	data.Manage.Boards, err = db.allBoards()
	if err != nil {
		log.Fatal(err)
	}
}
