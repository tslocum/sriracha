package server

import (
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveMod(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_mod"

	var postID int
	var action = "db"
	modInfo := PathString(r, "/sriracha/mod/")
	if modInfo != "" {
		split := strings.Split(modInfo, "/")
		if len(split) == 2 {
			switch split[0] {
			case "delete":
				action = "d"
			case "ban":
				action = "b"
			case "sticky":
				action = "s"
			case "unsticky":
				action = "us"
			case "lock":
				action = "l"
			case "unlock":
				action = "ul"
			case "view":
				action = "v"
			case "move":
				action = "m"
			default:
				data.ManageError("Unknown mod action")
				return
			}
			postID = ParseInt(split[1])
		} else if len(split) == 1 {
			postID = ParseInt(split[0])
		}
	}
	if postID == 0 {
		data.ManageError("Unknown post")
		return
	}
	data.Post = db.PostByID(postID)
	if data.Post == nil {
		data.ManageError("Unknown post")
		return
	}
	if action == "v" {
		if !s.opt.Identifiers {
			data.ManageError("Identifiers are not enabled")
			return
		}
		data.Template = "board_page"
		data.ModMode = true
		data.ReplyMode = 1
		data.Board = data.Post.Board
		posts := db.PostsByIP(data.Post.IP)
		if r.FormValue("confirmation") == "1" {
			for _, post := range posts {
				s.deletePost(db, post)
				s.log(db, data.Account, post.Board, fmt.Sprintf("Deleted No.%d", post.ID), "")
				s.rebuildThread(db, post)
			}
			data.Message = "Deleted all posts by author."
			return
		}
		for _, post := range posts {
			data.Threads = append(data.Threads, []*Post{post})
		}
		data.Message = `<form method="post" onsubmit="javascript:return confirm('Delete all posts by author?');"><input type="hidden" name="confirmation" value="1"><input type="submit" value="Delete all posts by author"></form><br>`
		return
	} else if action == "m" {
		if data.Post.Parent != 0 {
			data.ManageError("Only threads may be moved")
			return
		}
		data.Template = "board_page"
		data.ModMode = true
		data.ReplyMode = 1
		data.Board = data.Post.Board
		data.Threads = append(data.Threads, []*Post{data.Post})
		if r.FormValue("confirmation") == "1" {
			boardID := FormInt(r, "board")
			destination := db.BoardByID(boardID)
			if destination == nil {
				data.ManageError("Failed to move thread: Unknown board")
				return
			} else if destination.ID == data.Board.ID {
				data.ManageError("Failed to move thread: Thread is already located in selected board")
				return
			}
			posts := db.AllPostsInThread(data.Post.ID, false)
			// Verify attachments do not already exist at destination board.
			for _, p := range posts {
				if p.File != "" && !p.IsEmbed() {
					_, err := os.Stat(filepath.Join(s.config.Root, destination.Path(), "src", p.File))
					if err != nil && !os.IsNotExist(err) {
						data.ManageError(fmt.Sprintf("Failed to move thread: File /src/%s already exists at destination", p.File))
						return
					}
				}
				if p.Thumb != "" {
					_, err := os.Stat(filepath.Join(s.config.Root, destination.Path(), "thumb", p.File))
					if err != nil && !os.IsNotExist(err) {
						data.ManageError(fmt.Sprintf("Failed to move thread: File /thumb/%s already exists at destination", p.File))
						return
					}
				}
			}
			// Copy attachments.
			copyFile := func(dirName string, fileName string) error {
				srcPath := filepath.Join(s.config.Root, data.Post.Board.Path(), dirName, fileName)
				dstPath := filepath.Join(s.config.Root, destination.Path(), dirName, fileName)

				srcFile, err := os.Open(srcPath)
				if err != nil {
					return fmt.Errorf("Failed to move thread: Failed to open source file /%s/%s: %s", dirName, fileName, err)
				}
				dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
				if err != nil {
					srcFile.Close()
					return fmt.Errorf("Failed to move thread: Failed to open destination file /%s/%s: %s", dirName, fileName, err)
				}
				_, err = io.Copy(dstFile, srcFile)
				srcFile.Close()
				dstFile.Close()
				if err != nil {
					return fmt.Errorf("Failed to move thread: Failed to copy file /%s/%s: %s", dirName, fileName, err)
				}
				return nil
			}
			for _, p := range posts {
				if p.File != "" && !p.IsEmbed() {
					err := copyFile("src", p.File)
					if err != nil {
						data.ManageError(err.Error())
						return
					}
				}
				if p.Thumb != "" {
					err := copyFile("thumb", p.Thumb)
					if err != nil {
						data.ManageError(err.Error())
						return
					}
				}
			}
			// Remove source attachments.
			for _, p := range posts {
				if p.File != "" && !p.IsEmbed() {
					os.Remove(filepath.Join(s.config.Root, data.Post.Board.Path(), "src", p.File))
				}
				if p.Thumb != "" {
					os.Remove(filepath.Join(s.config.Root, data.Post.Board.Path(), "thumb", p.Thumb))
				}
			}
			// Delete thread page.
			os.Remove(filepath.Join(s.config.Root, data.Post.Board.Path(), "res", fmt.Sprintf("%d.html", data.Post.ID)))
			// Update post board.
			source := data.Post.Board
			for _, p := range posts {
				db.UpdatePostBoard(p.ID, destination.ID)
				p.Board = destination
				refPath := fmt.Sprintf("res/%d.html#%d", p.Thread(), p.ID)
				oldPath := source.Path() + refPath
				newPath := destination.Path() + refPath
				_, err := db.Exec(`UPDATE post SET message = replace(replace(message, '<a href="` + oldPath + `" class="refop">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>', '<a href="` + newPath + `" class="refop">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>'), '<a href="` + oldPath + `" class="refreply">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>', '<a href="` + newPath + `" class="refreply">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>') WHERE message LIKE '%&gt;&gt;` + strconv.Itoa(p.ID) + `%'`)
				if err != nil {
					log.Fatalf("failed to move thread: failed to update reflinks: %s", err)
				}
			}
			data.Post = db.PostByID(data.Post.ID)
			data.Board = destination
			data.Threads = [][]*Post{{data.Post}}
			data.Message = template.HTML(fmt.Sprintf("Moved No.%d to %s.", data.Post.ID, destination.Path()))
			s.log(db, data.Account, data.Post.Board, fmt.Sprintf("Moved >>/post/%d to >>/board/%d", data.Post.ID, data.Board.ID), "")
			// Add notice.
			if FormInt(r, "notice") == 1 {
				const linkFormat = `<a href="%s">&gt;&gt;&gt;%s</a>`
				sourceLink := fmt.Sprintf(linkFormat, source.Path(), source.Path())
				destinationLink := fmt.Sprintf(linkFormat, destination.Path(), destination.Path())
				now := time.Now().Unix()
				p := &Post{
					Board:     destination,
					Parent:    data.Post.ID,
					Timestamp: now,
					Bumped:    now,
					Message:   Get(destination, nil, "Thread moved from %[1]s to %[2]s.", sourceLink, destinationLink),
					Moderated: ModeratedApproved,
				}
				p.SetNameBlock("", "Mod", false)
				db.AddPost(p)
			}
			// Rebuild static files.
			s.rebuildThread(db, data.Post)
			s.writeIndexes(db, source)
		} else {
			moveLabel := Get(data.Board, data.Account, "Move")
			boardLabel := Get(data.Board, data.Account, "Board")
			data.Message = `<br><fieldset style="display: inline-block;"><legend>` + template.HTML(html.EscapeString(moveLabel)) + ` No.` + template.HTML(strconv.Itoa(data.Post.ID)) + `</legend><form method="post"><table border="0" class="manageform"><input type="hidden" name="confirmation" value="1"><tr><td class="postblock">` + template.HTML(html.EscapeString(boardLabel)) + `</td><td><select name="board">`
			for _, b := range db.AllBoards() {
				var extra string
				if data.Board.ID == b.ID {
					extra = " selected"
				}
				data.Message += template.HTML(fmt.Sprintf(`<option value="%d"%s>%s %s</option>`, b.ID, extra, b.Path(), html.EscapeString(b.Name)))
			}
			noticeLabel := Get(data.Board, data.Account, "Notice")
			addNoticeLabel := Get(data.Board, data.Account, "Add notice")
			data.Message += `</select></td></tr><tr><td class="postblock"><label for="notice">` + template.HTML(html.EscapeString(noticeLabel)) + `</label></td><td><label><input type="checkbox" id="notice" name="notice" value="1"> ` + template.HTML(html.EscapeString(addNoticeLabel)) + `</label></td></tr><tr><td>&nbsp;</td><td align="right"><input type="submit" class="managebutton" style="width: auto;min-width: 50%;" value="` + template.HTML(html.EscapeString(moveLabel)) + `"></td></tr></table></form></fieldset><br><br>`
		}
		return
	}
	threadAction := action == "s" || action == "us" || action == "l" || action == "ul"
	if threadAction {
		if data.Post.Parent != 0 {
			data.ManageError("Invalid post")
			return
		}

		var skipRebuild bool
		switch {
		case action == "s" && !data.Post.Stickied:
			db.StickyPost(data.Post.ID, true)
			s.log(db, data.Account, nil, fmt.Sprintf("Stickied >>/post/%d", data.Post.ID), "")
		case action == "us" && data.Post.Stickied:
			db.StickyPost(data.Post.ID, false)
			s.log(db, data.Account, nil, fmt.Sprintf("Unstickied >>/post/%d", data.Post.ID), "")
		case action == "l" && !data.Post.Locked:
			db.LockPost(data.Post.ID, true)
			s.log(db, data.Account, nil, fmt.Sprintf("Locked >>/post/%d", data.Post.ID), "")
		case action == "ul" && data.Post.Locked:
			db.LockPost(data.Post.ID, false)
			s.log(db, data.Account, nil, fmt.Sprintf("Unlocked >>/post/%d", data.Post.ID), "")
		default:
			skipRebuild = true
		}
		if !skipRebuild {
			s.rebuildThread(db, data.Post)
		}

		data.Template = "manage_info"
		http.Redirect(w, r, fmt.Sprintf("/sriracha/board/mod/%d/%d", data.Post.Board.ID, data.Post.ID), http.StatusFound)
		return
	}
	data.Board = data.Post.Board
	data.Threads = [][]*Post{{data.Post}}
	data.Manage.Ban = db.BanByIP(data.Post.IP)
	if r.FormValue("confirmation") == "1" {
		banFile := FormString(r, "banfile")
		if banFile != "" && !db.FileBanned(banFile) {
			db.AddFileBan(banFile)
			s.log(db, data.Account, nil, "Banned file", "")
		}

		var oldBan Ban
		if data.Manage.Ban != nil {
			oldBan = *data.Manage.Ban
		}
		if action == "b" || action == "db" {
			if data.Manage.Ban != nil {
				s.loadBanForm(db, r, data.Manage.Ban)
				db.UpdateBan(data.Manage.Ban)

				changes := printChanges(oldBan, *data.Manage.Ban)
				s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/ban/%d", data.Manage.Ban.ID), changes)
			} else {
				ban := &Ban{}
				s.loadBanForm(db, r, ban)
				ban.IP = data.Post.IP
				db.AddBan(ban)

				s.log(db, data.Account, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info())
			}
		}
		if action == "d" || action == "db" {
			s.deletePost(db, data.Post)

			s.log(db, data.Account, data.Board, fmt.Sprintf("Deleted No.%d", data.Post.ID), "")

			s.rebuildThread(db, data.Post)
		}

		label := "Deleted"
		switch action {
		case "b":
			label = "Banned"
		case "db":
			label = "Deleted and banned"
		}

		data.Template = "manage_info"
		data.Info = fmt.Sprintf("%s No.%d", label, data.Post.ID)
		return
	}

	data.ModMode = true
	data.ReplyMode = 1
	data.Extra = action
	if data.Post != nil {
		data.Extra2 = data.Post.FileHash
	}
}
