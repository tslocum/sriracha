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
	"slices"
	"strconv"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveMod(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_mod"

	var selected []*Post
	parsePosts := func(ids string) []*Post {
		var out []*Post
		for _, id := range strings.Split(ids, ",") {
			postID, err := strconv.Atoi(id)
			if err == nil && postID > 0 {
				post := db.PostByID(postID)
				if post != nil {
					out = append(out, post)
				}
			}
		}
		return out
	}
	var action string
	modInfo := PathString(r, "/sriracha/mod/")
	if modInfo != "" {
		split := strings.Split(modInfo, "/")
		if len(split) == 2 {
			switch split[0] {
			case "delete":
				action = "d"
			case "ban":
				action = "b"
			case "deleteban":
				action = "db"
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
			selected = parsePosts(split[1])
		} else if len(split) == 1 {
			selected = parsePosts(split[0])
		}
	}
	if len(selected) == 0 {
		data.ManageError("Unknown post")
		return
	}
	slices.SortFunc(selected, func(a *Post, b *Post) int {
		if a.ID < b.ID {
			return -1
		} else if a.ID > b.ID {
			return 1
		}
		return 0
	})
	if action == "v" {
		if !s.opt.Identifiers {
			data.ManageError("Identifiers are not enabled")
			return
		}
		data.Template = "board_page"
		data.ModMode = true
		data.ReplyMode = 1
		data.Board = selected[0].Board
		posts := db.PostsByIP(selected[0].IP)
		if r.FormValue("confirmation") == "1" {
			if s.forbidden(w, data, "post.delete") {
				return
			}
			for _, post := range posts {
				s.deletePost(db, post)
				s.log(db, data.Account, post.Board, fmt.Sprintf("Deleted >>%d", post.ID), "")
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
		if s.forbidden(w, data, "post.move") {
			return
		}
		post := selected[0]
		if post.Parent != 0 {
			data.ManageError("Only threads may be moved")
			return
		}
		data.Template = "board_page"
		data.ModMode = true
		data.ReplyMode = 1
		data.Board = post.Board
		data.Threads = append(data.Threads, []*Post{post})
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
			posts := db.AllPostsInThread(post.ID, false)
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
				srcPath := filepath.Join(s.config.Root, post.Board.Path(), dirName, fileName)
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
					os.Remove(filepath.Join(s.config.Root, post.Board.Path(), "src", p.File))
				}
				if p.Thumb != "" {
					os.Remove(filepath.Join(s.config.Root, post.Board.Path(), "thumb", p.Thumb))
				}
			}
			// Delete thread page.
			os.Remove(filepath.Join(s.config.Root, post.Board.Path(), "res", fmt.Sprintf("%d.html", post.ID)))
			// Update post board.
			source := post.Board
			for _, p := range posts {
				db.UpdatePostBoard(p.ID, destination)
				p.Board = destination
				refPath := fmt.Sprintf("res/%d.html#%d", p.Thread(), p.ID)
				oldPath := source.Path() + refPath
				newPath := destination.Path() + refPath
				_, err := db.Exec(`UPDATE post SET message = replace(replace(message, '<a href="` + oldPath + `" class="refop">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>', '<a href="` + newPath + `" class="refop">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>'), '<a href="` + oldPath + `" class="refreply">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>', '<a href="` + newPath + `" class="refreply">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>') WHERE message LIKE '%&gt;&gt;` + strconv.Itoa(p.ID) + `%'`)
				if err != nil {
					log.Fatalf("failed to move thread: failed to update reflinks: %s", err)
				}
			}
			post = db.PostByID(post.ID)
			data.Board = destination
			data.Threads = [][]*Post{{post}}
			data.Message = template.HTML(fmt.Sprintf("Moved No.%d to %s.", post.ID, destination.Path()))
			s.log(db, data.Account, post.Board, fmt.Sprintf("Moved >>/post/%d to >>/board/%d", post.ID, data.Board.ID), "")
			// Add notice.
			if FormInt(r, "notice") == 1 {
				const linkFormat = `<a href="%s">&gt;&gt;&gt;%s</a>`
				sourceLink := fmt.Sprintf(linkFormat, source.Path(), source.Path())
				destinationLink := fmt.Sprintf(linkFormat, destination.Path(), destination.Path())
				now := time.Now().Unix()
				p := &Post{
					Board:     destination,
					Parent:    post.ID,
					Timestamp: now,
					Bumped:    now,
					Message:   Get(destination, nil, "Thread moved from %[1]s to %[2]s.", sourceLink, destinationLink),
					Moderated: ModeratedApproved,
				}
				p.SetNameBlock("", "Mod", false)
				db.AddPost(p)
			}
			// Rebuild static files.
			s.rebuildThread(db, post)
			s.writeBoardIndexes(db, source)
		} else {
			moveLabel := Get(data.Board, data.Account, "Move")
			boardLabel := Get(data.Board, data.Account, "Board")
			data.Message = `<br><fieldset><legend>` + template.HTML(html.EscapeString(moveLabel)) + ` No.` + template.HTML(strconv.Itoa(post.ID)) + `</legend><form method="post"><table class="manageform"><input type="hidden" name="confirmation" value="1"><tr><td class="postblock">` + template.HTML(html.EscapeString(boardLabel)) + `</td><td><select name="board">`
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
		post := selected[0]
		if post.Parent != 0 {
			data.ManageError("Invalid post")
			return
		}

		var skipRebuild bool
		switch {
		case action == "s" && !post.Stickied:
			if s.forbidden(w, data, "post.sticky") {
				return
			}
			db.StickyPost(post.ID, true)
			s.log(db, data.Account, nil, fmt.Sprintf("Stickied >>/post/%d", post.ID), "")
		case action == "us" && post.Stickied:
			if s.forbidden(w, data, "post.sticky") {
				return
			}
			db.StickyPost(post.ID, false)
			s.log(db, data.Account, nil, fmt.Sprintf("Unstickied >>/post/%d", post.ID), "")
		case action == "l" && !post.Locked:
			if s.forbidden(w, data, "post.lock") {
				return
			}
			db.LockPost(post.ID, true)
			s.log(db, data.Account, nil, fmt.Sprintf("Locked >>/post/%d", post.ID), "")
		case action == "ul" && post.Locked:
			if s.forbidden(w, data, "post.lock") {
				return
			}
			db.LockPost(post.ID, false)
			s.log(db, data.Account, nil, fmt.Sprintf("Unlocked >>/post/%d", post.ID), "")
		default:
			skipRebuild = true
		}
		if !skipRebuild {
			s.rebuildThread(db, post)
		}

		data.Template = "manage_info"
		data.Redirect(w, r, fmt.Sprintf("/sriracha/board/mod/%d/%d", post.Board.ID, post.ID))
		return
	}
	data.Board = selected[0].Board
	data.Threads = [][]*Post{selected}
	if r.FormValue("confirmation") == "1" {
		if s.forbidden(w, data, "ban.add") || s.forbidden(w, data, "ban.lengthen") || s.forbidden(w, data, "post.delete") {
			return
		}
		banFile := FormBool(r, "banfile")
		if banFile && s.forbidden(w, data, "banfile.add") {
			return
		}
		var rebuild [][2]int
		slices.Reverse(selected)
		for _, post := range selected {
			if banFile && post.FileHash != "" && !db.FileBanned(post.FileHash) {
				db.AddFileBan(post.FileHash)
				s.log(db, data.Account, nil, "Banned file", "")
			}

			var oldBan Ban
			ban := db.BanByIP(post.IP)
			if ban != nil {
				oldBan = *ban
			}
			if action == "b" || action == "db" {
				if ban != nil {
					if oldBan.Expire == 0 {
						continue
					}
					err := s.loadBanForm(db, r, ban)
					if err != nil {
						data.ManageError(err.Error())
						return
					} else if ban.Expire != 0 && ban.Expire <= oldBan.Expire {
						continue
					}
					db.UpdateBan(ban)

					changes := printChanges(oldBan, *ban)
					s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/ban/%d", ban.ID), changes)
				} else {
					ban := &Ban{}
					err := s.loadBanForm(db, r, ban)
					if err != nil {
						data.ManageError(err.Error())
						return
					}
					ban.IP = post.IP
					db.AddBan(ban)

					s.log(db, data.Account, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info())
				}
			}
			if action == "d" || action == "db" {
				s.deletePost(db, post)

				s.log(db, data.Account, data.Board, fmt.Sprintf("Deleted >>%d", post.ID), "")

				info := [2]int{post.Board.ID, post.Thread()}
				if !slices.Contains(rebuild, info) {
					rebuild = append(rebuild, info)
				}
			}
		}

		var boards []int
		for _, info := range rebuild {
			post := db.PostByID(info[1])
			if post != nil {
				s.writeThread(db, post.Board, post.ID)
			}
			if !slices.Contains(boards, info[0]) {
				boards = append(boards, info[0])
			}
		}
		for _, boardID := range boards {
			board := db.BoardByID(boardID)
			if board == nil {
				continue
			}
			s.writeBoardIndexes(db, board)
		}
		if s.opt.Overboard != "" {
			s.writeOverboard(db)
		}
		s.writeSiteIndex(db)
		s.writeStatistics(db)
		s.writeModQueue(db)

		data.Template = "manage_info"
		switch action {
		case "d":
			data.Info = GetN(nil, data.Account, "Deleted %d post", "Deleted %d posts", len(selected))
		case "b":
			data.Info = GetN(nil, data.Account, "Banned %d post", "Banned %d posts", len(selected))
		default:
			data.Info = GetN(nil, data.Account, "Deleted & banned %d post", "Deleted & banned %d posts", len(selected))
		}
		return
	}

	data.ModMode = true
	data.ReplyMode = 1
	data.Extra = action
	existing := make(map[int][]*Post)
	var lastAddress string
	var multipleAddresses bool
	for _, thread := range data.Threads {
		for _, post := range thread {
			if data.Extra3 != "" {
				data.Extra3 += ","
			}
			data.Extra3 += strconv.Itoa(post.ID)

			if post.IP != lastAddress {
				if lastAddress == "" {
					lastAddress = post.IP
				} else {
					multipleAddresses = true
				}
			}

			ban := db.BanByIP(post.IP)
			if ban == nil {
				continue
			}
			existing[ban.ID] = append(existing[ban.ID], post)
		}
	}
	for banID, posts := range existing {
		ban := db.BanByID(banID)
		if ban == nil {
			continue
		}
		data.Message += `<tr><td align="right">`
		for i, post := range posts {
			if i != 0 {
				data.Message += ", "
			}
			data.Message += post.RefLink()
		}
		data.Message += `</td>
		<td>` + template.HTML(ban.Info()) + `</td><td><form method="get" action="/sriracha/ban/` + template.HTML(strconv.Itoa(banID)) + `"><input type="submit" value="` + template.HTML(G(nil, data.Account, "Update")) + `"></form>`
		var ids []int
		for _, post := range selected {
			var found bool
			for _, p := range posts {
				if p.ID == post.ID {
					found = true
					break
				}
			}
			if !found {
				ids = append(ids, post.ID)
			}
		}
		if len(ids) > 0 {
			method := template.HTML("get")
			if action == "db" {
				method = "post"
			}
			data.Message += ` <form method="` + method + `" action="/sriracha/mod/`
			switch action {
			case "d":
				data.Message += "delete/"
			case "b":
				data.Message += "ban/"
			case "db":
				data.Message += "deleteban/"
			}
			for i, id := range ids {
				if i != 0 {
					data.Message += ","
				}
				data.Message += template.HTML(strconv.Itoa(id))
			}
			data.Message += `"><input type="submit" value="` + template.HTML(G(nil, data.Account, "Deselect")) + `"></form>`
		}
		data.Message += `</td></tr>`
	}
	if len(existing) > 0 {
		data.Message = `<fieldset><legend>` + template.HTML(GetN(nil, data.Account, "%d existing ban", "%d existing bans", len(existing))) + `</legend><table>` + data.Message + `</table></fieldset>`
	}
	if !multipleAddresses {
		data.Manage.LiftedBans = db.LiftedBansByIP(data.Threads[0][0].IP)
	}
}
