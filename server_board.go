package sriracha

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func (s *Server) serveBoard(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) (skipExecute bool) {
	data.Template = "manage_board"

	boardID := pathInt(r, "/sriracha/board/rebuild/")
	if boardID > 0 {
		if data.forbidden(w, RoleAdmin) {
			return false
		}
		b := db.boardByID(boardID)
		if b == nil {
			data.ManageError("Board not found")
			return false
		}
		s.rebuildBoard(db, b)
		data.Info = fmt.Sprintf("Rebuilt %s", b.Path())
	}

	modBoard := pathString(r, "/sriracha/board/mod/")
	if modBoard != "" {
		var postID int
		var page int
		split := strings.Split(modBoard, "/")
		if len(split) == 2 {
			boardID, _ = strconv.Atoi(split[0])
			if strings.HasPrefix(split[1], "p") {
				page = parseInt(split[1][1:])
			} else {
				postID = parseInt(split[1])
			}
		} else if len(split) == 1 {
			boardID, _ = strconv.Atoi(split[0])
		}

		b := db.boardByID(boardID)
		if b == nil {
			data.ManageError("Invalid or deleted board or post")
			return false
		}

		data.Template = "board_page"
		data.Board = b
		data.Boards = db.allBoards()
		data.ModMode = true
		if postID > 0 {
			data.Threads = [][]*Post{db.allPostsInThread(postID, true)}
			data.ReplyMode = postID
		} else {
			threads := db.allThreads(b, true)

			pages := 1
			if len(threads) != 0 && b.Threads != 0 {
				pages = len(threads) / b.Threads
				if len(threads)%b.Threads != 0 {
					pages++
				}
			}
			data.Page = page
			data.Pages = pages

			start := page * b.Threads
			end := len(threads)
			if b.Threads != 0 && end > start+b.Threads {
				end = start + b.Threads
			}
			for _, thread := range threads[start:end] {
				data.Threads = append(data.Threads, db.allPostsInThread(thread.ID, true))
			}
		}
		return false
	}

	boardID = pathInt(r, "/sriracha/board/")
	if boardID > 0 {
		data.Manage.Board = db.boardByID(boardID)
		if data.Manage.Board == nil {
			data.ManageError("Board not found")
			return false
		}

		if data.Manage.Board != nil && r.Method == http.MethodPost {
			if data.forbidden(w, RoleAdmin) {
				return false
			}
			oldBoard := *data.Manage.Board

			oldDir := data.Manage.Board.Dir
			oldPath := data.Manage.Board.Path()
			data.Manage.Board.loadForm(r, s.config.UploadTypes(), s.opt.Embeds)

			err := data.Manage.Board.validate()
			if err != nil {
				data.ManageError(err.Error())
				return false
			}

			if data.Manage.Board.Dir != "" && data.Manage.Board.Dir != oldDir {
				_, err := os.Stat(filepath.Join(s.config.Root, data.Manage.Board.Dir))
				if err != nil {
					if !os.IsNotExist(err) {
						log.Fatal(err)
					}
				} else {
					data.ManageError("New directory already exists")
					return false
				}
			}

			db.updateBoard(data.Manage.Board)

			if data.Manage.Board.Dir != oldDir {
				subDirs := []string{"src", "thumb", "res"}
				for _, subDir := range subDirs {
					newPath := filepath.Join(s.config.Root, data.Manage.Board.Dir, subDir)
					_, err := os.Stat(newPath)
					if err == nil {
						data.ManageError(fmt.Sprintf("New board directory %s already exists", newPath))
						return false
					}
				}
				moveSubDirs := func() error {
					for _, subDir := range subDirs {
						oldPath := filepath.Join(s.config.Root, oldDir, subDir)
						newPath := filepath.Join(s.config.Root, data.Manage.Board.Dir, subDir)
						err := os.Rename(oldPath, newPath)
						if err != nil {
							return fmt.Errorf("Failed to rename board directory %s to %s: %s", oldPath, newPath, err)
						}
					}
					return nil
				}
				if data.Manage.Board.Dir == "" {
					err = moveSubDirs()
					if err != nil {
						data.ManageError(err.Error())
						return false
					}
				} else {
					if oldDir == "" {
						err := os.Mkdir(filepath.Join(s.config.Root, data.Manage.Board.Dir), newDirPermission)
						if err != nil {
							data.ManageError(fmt.Sprintf("Failed to create board directory: %s", err))
							return false
						}
						err = moveSubDirs()
						if err != nil {
							data.ManageError(err.Error())
							return false
						}
					} else {
						err := os.Rename(filepath.Join(s.config.Root, oldDir), filepath.Join(s.config.Root, data.Manage.Board.Dir))
						if err != nil {
							data.ManageError(fmt.Sprintf("Failed to rename board directory: %s", err))
							return false
						}
					}
				}

				for _, thread := range db.allThreads(data.Manage.Board, false) {
					for _, post := range db.allPostsInThread(thread.ID, false) {
						var modified bool
						resPattern, err := regexp.Compile(`<a href="` + regexp.QuoteMeta(oldPath) + `res\/([0-9]+).html#([0-9]+)"`)
						if err != nil {
							log.Fatalf("failed to compile res pattern: %s", err)
						}
						post.Message = resPattern.ReplaceAllStringFunc(post.Message, func(s string) string {
							modified = true
							match := resPattern.FindStringSubmatch(s)
							return fmt.Sprintf(`<a href="%sres/%s.html#%s"`, data.Manage.Board.Path(), match[1], match[2])
						})
						if modified {
							db.updatePostMessage(post.ID, post.Message)
						}
					}
				}
			}

			s.rebuildBoard(db, data.Manage.Board)

			changes := printChanges(oldBoard, *data.Manage.Board)
			db.log(data.Account, nil, fmt.Sprintf("Updated >>/board/%d", data.Manage.Board.ID), changes)

			http.Redirect(w, r, "/sriracha/board/", http.StatusFound)
			return true
		}
		return false
	}

	if r.Method == http.MethodPost {
		if data.forbidden(w, RoleAdmin) {
			return
		}
		b := &Board{}
		b.loadForm(r, s.config.UploadTypes(), s.opt.Embeds)

		err := b.validate()
		if err != nil {
			data.ManageError(err.Error())
			return false
		}

		dirs := []string{"", "src", "thumb", "res"}
		for _, boardDir := range dirs {
			if b.Dir == "" && boardDir == "" {
				continue
			}
			boardPath := filepath.Join(s.config.Root, b.Dir, boardDir)
			err = os.Mkdir(boardPath, newDirPermission)
			if err != nil {
				if os.IsExist(err) {
					data.ManageError(fmt.Sprintf("Board directory %s already exists.", boardPath))
				} else {
					data.ManageError(fmt.Sprintf("Failed to create board directory %s: %s", boardPath, err))
				}
				return false
			}
		}

		db.addBoard(b)

		s.rebuildBoard(db, b)

		db.log(data.Account, nil, fmt.Sprintf("Added >>/board/%d", b.ID), "")

		http.Redirect(w, r, "/sriracha/board/", http.StatusFound)
		return true
	}

	data.Manage.Board = newBoard()

	data.Manage.Boards = db.allBoards()
	return false
}
