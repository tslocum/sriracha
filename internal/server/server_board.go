package server

import (
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadBoardForm(db *database.DB, r *http.Request, b *Board) {
	b.Dir = FormString(r, "dir")
	b.Name = FormString(r, "name")
	b.Description = FormString(r, "description")
	b.Type = FormRange(r, "type", TypeImageboard, TypeForum)
	b.Hide = FormRange(r, "hide", HideNowhere, HideEverywhere)
	b.Lock = FormRange(r, "lock", LockNone, LockStaff)
	b.Approval = FormRange(r, "approval", ApprovalNone, ApprovalAll)
	b.Reports = FormBool(r, "reports")
	b.Style = FormString(r, "style")
	b.Locale = FormString(r, "locale")
	b.Delay = FormInt(r, "delay")
	b.MinName = FormInt(r, "minname")
	b.MaxName = FormInt(r, "maxname")
	b.MinEmail = FormInt(r, "minemail")
	b.MaxEmail = FormInt(r, "maxemail")
	b.MinSubject = FormInt(r, "minsubject")
	b.MaxSubject = FormInt(r, "maxsubject")
	b.MinMessage = FormInt(r, "minmessage")
	b.MaxMessage = FormInt(r, "maxmessage")
	b.MinSizeThread = FormInt64(r, "minsizethread")
	b.MaxSizeThread = FormInt64(r, "maxsizethread")
	b.MinSizeReply = FormInt64(r, "minsizereply")
	b.MaxSizeReply = FormInt64(r, "maxsizereply")
	b.ThumbWidth = FormInt(r, "thumbwidth")
	b.ThumbHeight = FormInt(r, "thumbheight")
	b.DefaultName = FormString(r, "defaultname")
	b.WordBreak = FormInt(r, "wordbreak")
	b.Truncate = FormInt(r, "truncate")
	b.Threads = FormInt(r, "threads")
	b.Replies = FormInt(r, "replies")
	b.MaxThreads = FormInt(r, "maxthreads")
	b.MaxReplies = FormInt(r, "maxreplies")
	b.Oekaki = FormBool(r, "oekaki")
	b.Rules = FormMultiString(r, "rules")
	b.Backlinks = FormBool(r, "backlinks")
	b.Instances = FormNegInt(r, "instances")
	b.Identifiers = FormRange(r, "identifiers", IdentifiersDisable, IdentifiersGlobal)
	b.Files = FormInt(r, "files")
	b.Gallery = FormBool(r, "gallery")

	if b.Locale != "" && !slices.Contains(s.opt.LocalesSorted, b.Locale) {
		b.Locale = ""
	}

	if b.Files < 0 {
		b.Files = 0
	}

	b.Uploads = nil
	uploads := r.Form["uploads"]
	availableUploads := s.config.UploadTypes()
	for _, upload := range uploads {
		var found bool
		for _, u := range availableUploads {
			if u.MIME == upload {
				found = true
				break
			}
		}
		if found {
			b.Uploads = append(b.Uploads, upload)
		}
	}

	b.Embeds = nil
	embeds := r.Form["embeds"]
	for _, embed := range embeds {
		var found bool
		for _, info := range s.opt.Embeds {
			if info[0] == embed {
				found = true
				break
			}
		}
		if found {
			b.Embeds = append(b.Embeds, embed)
		}
	}
}

func (s *Server) serveBoard(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) (skipExecute bool) {
	data.Template = "manage_board"

	boardID := PathInt(r, "/sriracha/board/rebuild/")
	if boardID > 0 {
		if data.forbidden(w, RoleAdmin) {
			return false
		}
		b := db.BoardByID(boardID)
		if b == nil {
			data.ManageError("Board not found")
			return false
		}
		s.rebuildBoard(db, b)
		data.Info = fmt.Sprintf("Rebuilt %s", b.Path())
	}

	modBoard := PathString(r, "/sriracha/board/mod/")
	if modBoard != "" {
		var postID int
		var page int
		split := strings.Split(modBoard, "/")
		if len(split) == 2 {
			boardID, _ = strconv.Atoi(split[0])
			if strings.HasPrefix(split[1], "p") {
				page = ParseInt(split[1][1:])
			} else {
				postID = ParseInt(split[1])
			}
		} else if len(split) == 1 {
			boardID, _ = strconv.Atoi(split[0])
		}

		b := db.BoardByID(boardID)
		if b == nil {
			data.ManageError("Invalid or deleted board or post")
			return false
		}

		data.Template = "board_page"
		data.Board = b
		data.Boards = db.AllBoards()
		data.ModMode = true
		if postID > 0 {
			data.Threads = [][]*Post{db.AllPostsInThread(postID, true)}
			data.ReplyMode = postID
		} else {
			allThreads := db.AllThreads(b, true)

			data.Page = page
			data.Pages = pageCount(len(allThreads), b.Threads)

			start := page * b.Threads
			end := len(allThreads)
			if b.Threads != 0 && end > start+b.Threads {
				end = start + b.Threads
			}
			for _, threadInfo := range allThreads[start:end] {
				thread := db.PostByID(threadInfo[0])
				thread.Replies = threadInfo[1]
				posts := []*Post{thread}
				if b.Type == TypeImageboard {
					posts = append(posts, db.AllReplies(threadInfo[0], b.Replies, true)...)
				}
				data.Threads = append(data.Threads, posts)
			}
		}
		return false
	}

	deleteBoardID := PathInt(r, "/sriracha/board/delete/")
	if deleteBoardID > 0 {
		if s.forbidden(w, data, "board.delete") {
			return
		}

		b := db.BoardByID(deleteBoardID)
		if b == nil {
			data.ManageError("Invalid board.")
			return
		}

		allThreads := db.AllThreads(b, false)
		if !FormBool(r, "confirmation") {
			data.Template = "manage_info"
			data.Message = template.HTML(`<form method="post">
			<input type="hidden" name="confirmation" value="1">
			<fieldset>
				<legend>
					Delete ` + b.Path() + ` ` + html.EscapeString(b.Name) + `
				</legend>
				<div>
					<h1>WARNING!</h1>
					You are about to <b>PERMANENTLY DELETE</b> ` + b.Path() + ` ` + html.EscapeString(b.Name) + `!<br>
					` + strconv.Itoa(len(allThreads)) + ` threads in ` + b.Path() + ` will be <b>permanently deleted</b>.<br>
					This operation cannot be undone.<br><br>
					<input type="submit" value="Delete ` + b.Path() + `">
				</div>
			</fieldset>
			</form>`)
			return
		}
		for _, threadInfo := range allThreads {
			s.deletePost(db, db.PostByID(threadInfo[0]))
		}
		db.DeleteBoard(b.ID)

		if b.Dir != "" {
			var skipDeleteDir bool
			boardPath := filepath.Join(s.config.Root, b.Dir)
			pattern := regexp.MustCompile(`^(index|catalog|[0-9]+).html$`)
			filepath.WalkDir(boardPath, func(path string, d fs.DirEntry, err error) error {
				if !d.IsDir() && !pattern.MatchString(d.Name()) && err == nil {
					skipDeleteDir = true
					return filepath.SkipAll
				}
				return nil
			})
			if !skipDeleteDir {
				os.RemoveAll(boardPath)
			}
		}

		s.writeSiteIndex(db)

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted board #%d", b.ID), "")

		data.Template = "manage_info"
		http.Redirect(w, r, "/sriracha/board/", http.StatusFound)
		return
	}

	boardID = PathInt(r, "/sriracha/board/")
	if boardID > 0 {
		data.Manage.Board = db.BoardByID(boardID)
		if data.Manage.Board == nil {
			data.ManageError("Board not found")
			return false
		}

		if data.Manage.Board != nil && r.Method == http.MethodPost {
			if s.forbidden(w, data, "board.update") {
				return false
			}
			oldBoard := *data.Manage.Board

			oldDir := data.Manage.Board.Dir
			oldPath := data.Manage.Board.Path()
			s.loadBoardForm(db, r, data.Manage.Board)

			err := data.Manage.Board.Validate()
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

			db.UpdateBoard(data.Manage.Board)

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
						err := os.Mkdir(filepath.Join(s.config.Root, data.Manage.Board.Dir), NewDirPermission)
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

				for _, info := range db.AllThreads(data.Manage.Board, false) {
					for _, post := range db.AllPostsInThread(info[0], false) {
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
							db.UpdatePostMessage(post.ID, post.Message)
						}
					}
				}
			}

			s.rebuildBoard(db, data.Manage.Board)
			s.writeSiteIndex(db)

			changes := printChanges(oldBoard, *data.Manage.Board)
			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/board/%d", data.Manage.Board.ID), changes)

			http.Redirect(w, r, "/sriracha/board/", http.StatusFound)
			return true
		}
		return false
	}

	if r.Method == http.MethodPost {
		if s.forbidden(w, data, "board.add") {
			return
		}
		b := &Board{}
		s.loadBoardForm(db, r, b)

		if FormBool(r, "duplicate") {
			duplicateID := FormInt(r, "board")
			d := db.BoardByID(duplicateID)
			if d == nil {
				data.ManageError("Board not found")
				return false
			}
			b.Type = d.Type
			b.Hide = d.Hide
			b.Lock = d.Lock
			b.Approval = d.Approval
			b.Reports = d.Reports
			b.Style = d.Style
			b.Locale = d.Locale
			b.Delay = d.Delay
			b.MinName = d.MinName
			b.MaxName = d.MaxName
			b.MinEmail = d.MinEmail
			b.MaxEmail = d.MaxEmail
			b.MinSubject = d.MinSubject
			b.MaxSubject = d.MaxSubject
			b.MinMessage = d.MinMessage
			b.MaxMessage = d.MaxMessage
			b.MinSizeThread = d.MinSizeThread
			b.MaxSizeThread = d.MaxSizeThread
			b.MinSizeReply = d.MinSizeReply
			b.MaxSizeReply = d.MaxSizeReply
			b.ThumbWidth = d.ThumbWidth
			b.ThumbHeight = d.ThumbHeight
			b.DefaultName = d.DefaultName
			b.WordBreak = d.WordBreak
			b.Truncate = d.Truncate
			b.Threads = d.Threads
			b.Replies = d.Replies
			b.MaxThreads = d.MaxThreads
			b.MaxReplies = d.MaxReplies
			b.Oekaki = d.Oekaki
			b.Backlinks = d.Backlinks
			b.Files = d.Files
			b.Instances = d.Instances
			b.Identifiers = d.Identifiers
			b.Gallery = d.Gallery
			b.Uploads = d.Uploads
			b.Embeds = d.Embeds
			b.Rules = d.Rules
		}

		err := b.Validate()
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
			err = os.Mkdir(boardPath, NewDirPermission)
			if err != nil {
				if os.IsExist(err) {
					data.ManageError(fmt.Sprintf("Board directory %s already exists.", boardPath))
				} else {
					data.ManageError(fmt.Sprintf("Failed to create board directory %s: %s", boardPath, err))
				}
				return false
			}
		}

		db.AddBoard(b)

		s.rebuildBoard(db, b)
		s.writeSiteIndex(db)

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/board/%d", b.ID), "")

		http.Redirect(w, r, "/sriracha/board/", http.StatusFound)
		return true
	}

	data.Manage.Board = NewBoard()

	data.Manage.Boards = db.AllBoards()
	return false
}
