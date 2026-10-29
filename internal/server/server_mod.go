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
	"sync"
	"sync/atomic"
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
			case "deleteattachment":
				action = "da"
			case "lock":
				action = "l"
			case "unlock":
				action = "ul"
			case "sticky":
				action = "s"
			case "unsticky":
				action = "us"
			case "spoiler":
				action = "sp"
			case "unspoiler":
				action = "usp"
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
			var threadIDs []int
			for _, post := range posts {
				s.deletePost(db, post)
				s.log(db, data.Account, post.Board, fmt.Sprintf("Deleted >>%d", post.ID), "Deleted all posts by author.")
				threadID := post.Thread()
				if !slices.Contains(threadIDs, threadID) {
					threadIDs = append(threadIDs, threadID)
				}
			}

			for _, threadID := range threadIDs {
				s.unBumpThread(db, threadID)
			}

			wg := &sync.WaitGroup{}
			delta := &atomic.Int32{}
			db.SoftCommit()
			s.rebuildThreads(db, wg, delta, posts)
			wg.Wait()
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
		data.Template = "board_page"
		data.ModMode = true
		data.ReplyMode = 1
		checkAttachment := func(p *Post, destination *Board) error {
			if p.File != "" && !p.IsEmbed() {
				_, err := os.Stat(filepath.Join(s.config.Root, destination.Path(), "src", p.File))
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("Failed to move post: File /src/%s already exists at destination", p.File)
				}
			}
			if p.Thumb != "" {
				_, err := os.Stat(filepath.Join(s.config.Root, destination.Path(), "thumb", p.File))
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("Failed to move post: File /thumb/%s already exists at destination", p.File)
				}
			}
			return nil
		}
		copyFile := func(p *Post, destination *Board, dirName string, fileName string) error {
			srcPath := filepath.Join(s.config.Root, p.Board.Path(), dirName, fileName)
			dstPath := filepath.Join(s.config.Root, destination.Path(), dirName, fileName)

			srcFile, err := os.Open(srcPath)
			if err != nil {
				return fmt.Errorf("Failed to move post: Failed to open source file /%s/%s: %s", dirName, fileName, err)
			}
			dstFile, err := os.OpenFile(dstPath, NewFileFlags, NewFilePermission)
			if err != nil {
				srcFile.Close()
				return fmt.Errorf("Failed to move post: Failed to open destination file /%s/%s: %s", dirName, fileName, err)
			}
			_, err = io.Copy(dstFile, srcFile)
			srcFile.Close()
			dstFile.Close()
			if err != nil {
				return fmt.Errorf("Failed to move post: Failed to copy file /%s/%s: %s", dirName, fileName, err)
			}
			return nil
		}
		copyAttachment := func(p *Post, destination *Board) error {
			if p.File != "" && !p.IsEmbed() {
				err := copyFile(p, destination, "src", p.File)
				if err != nil {
					return err
				}
			}
			if p.Thumb != "" {
				err := copyFile(p, destination, "thumb", p.Thumb)
				if err != nil {
					return err
				}
			}
			return nil
		}
		removeSourceAttachment := func(p *Post) {
			if p.File != "" && !p.IsEmbed() {
				os.Remove(filepath.Join(s.config.Root, p.Board.Path(), "src", p.File))
			}
			if p.Thumb != "" {
				os.Remove(filepath.Join(s.config.Root, p.Board.Path(), "thumb", p.Thumb))
			}
		}
		removeThreadPage := func(p *Post) {
			os.Remove(filepath.Join(s.config.Root, p.Board.Path(), "res", fmt.Sprintf("%d.html", p.ID)))
			s.pageTimingLock.Lock()
			delete(s.pageTimings, p.Board.Path()+fmt.Sprintf("res/%d.html", p.ID))
			s.pageTimingLock.Unlock()
		}
		updatePost := func(p *Post, destination *Board, destinationThread int) {
			source := p.Board
			oldPath := source.Path() + fmt.Sprintf("res/%d.html#%d", p.Thread(), p.ID)
			if source.ID != destination.ID {
				db.UpdatePostBoard(p.ID, destination)
				p.Board = destination
			}
			if destinationThread == 0 {
				destinationThread = p.Thread()
			}
			newRefLabel := strconv.Itoa(p.ID)
			if destinationThread != p.Thread() {
				db.UpdatePostParent(p.ID, destinationThread)
				p.Parent = destinationThread
			} else {
				newRefLabel += "(OP)"
			}
			newPath := destination.Path() + fmt.Sprintf("res/%d.html#%d", destinationThread, p.ID)
			_, err := db.Exec(`UPDATE post SET message = replace(replace(message, '<a href="` + oldPath + `" class="refop">&gt;&gt;` + strconv.Itoa(p.ID) + `(OP)</a>', '<a href="` + newPath + `" class="refop">&gt;&gt;` + newRefLabel + `</a>'), '<a href="` + oldPath + `" class="refreply">&gt;&gt;` + strconv.Itoa(p.ID) + `(OP)</a>', '<a href="` + newPath + `" class="refreply">&gt;&gt;` + newRefLabel + `</a>') WHERE message LIKE '%&gt;&gt;` + strconv.Itoa(p.ID) + `%'`)
			if err != nil {
				log.Fatalf("failed to move post: failed to update reflinks: %s", err)
			}
			_, err = db.Exec(`UPDATE post SET message = replace(replace(message, '<a href="` + oldPath + `" class="refop">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>', '<a href="` + newPath + `" class="refop">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>'), '<a href="` + oldPath + `" class="refreply">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>', '<a href="` + newPath + `" class="refreply">&gt;&gt;` + strconv.Itoa(p.ID) + `</a>') WHERE message LIKE '%&gt;&gt;` + strconv.Itoa(p.ID) + `%'`)
			if err != nil {
				log.Fatalf("failed to move post: failed to update reflinks: %s", err)
			}
		}
		post := selected[0]
		data.Board = post.Board
		if FormInt(r, "thread") > 0 {
			var posts []*Post
			var postIDs []int
			for _, p := range selected {
				if p.Parent == 0 {
					for _, pp := range db.AllPostsInThread(FilterAny, p.ID) {
						if !slices.Contains(postIDs, pp.ID) {
							posts = append(posts, pp)
							postIDs = append(postIDs, pp.ID)
						}
					}
				} else if !slices.Contains(postIDs, p.ID) {
					posts = append(posts, p)
					postIDs = append(postIDs, p.ID)
				}
			}
			selected = posts
			threadID := FormInt(r, "thread")
			for _, p := range selected {
				if p.Thread() == threadID {
					data.ManageError("Failed to move post: That post is already located in selected thread")
					return
				}
			}
			destination := db.PostByID(threadID)
			if destination == nil || destination.Parent != 0 {
				data.ManageError("Failed to move post: Invalid destination thread")
				return
			}
			var threadIDs []int
			for _, p := range selected {
				if !slices.Contains(threadIDs, p.Thread()) {
					threadIDs = append(threadIDs, p.Thread())
				}
			}
			// Verify all posts are newer than the destination thread post.
			for _, p := range selected {
				if p.ID < threadID {
					data.ManageError("Failed to move post: Destination thread is newer than selected post")
					return
				}
			}
			// Verify attachments do not already exist at destination board.
			for _, p := range selected {
				if p.Board.ID != destination.Board.ID {
					err := checkAttachment(p, destination.Board)
					if err != nil {
						data.ManageError(err.Error())
						return
					}
				}
			}
			// Copy attachments.
			for _, p := range selected {
				if p.Board.ID != destination.Board.ID {
					err := copyAttachment(p, destination.Board)
					if err != nil {
						data.ManageError(err.Error())
						return
					}
				}
			}
			// Remove source attachments.
			for _, p := range selected {
				if p.Board.ID != destination.Board.ID {
					removeSourceAttachment(p)
				}
			}
			// Delete thread page.
			for _, p := range selected {
				if p.Parent == 0 {
					removeThreadPage(p)
				}
			}
			// Update posts.
			for _, p := range selected {
				updatePost(p, destination.Board, threadID)
			}
			for _, p := range selected {
				s.log(db, data.Account, post.Board, fmt.Sprintf("Moved >>/post/%d to >>/post/%d", p.ID, threadID), "")
			}
			// Rebuild static files.
			db.SoftCommit()
			var threads []*Post
			for _, id := range threadIDs {
				threads = append(threads, db.PostByID(id))
			}
			wg := &sync.WaitGroup{}
			delta := &atomic.Int32{}
			s.rebuildThreads(db, wg, delta, threads)
			wg.Wait()
			data.Redirect(w, r, fmt.Sprintf("/sriracha/board/mod/%d/%d", destination.Board.ID, destination.ID))
			// TODO backlinks, update reflinks in backlinking posts (including posts in and out of destination thread)
			// TODO bump thread when moving posts newer than bumped timestamp
		} else if FormInt(r, "board") > 0 {
			if post.Parent != 0 {
				data.ManageError("Only threads may be moved to another board")
				return
			}
			data.Threads = append(data.Threads, []*Post{post})
			boardID := FormInt(r, "board")
			destination := db.BoardByID(boardID)
			if destination == nil {
				data.ManageError("Failed to move thread: Unknown board")
				return
			} else if destination.ID == data.Board.ID {
				data.ManageError("Failed to move thread: Thread is already located in selected board")
				return
			}
			posts := db.AllPostsInThread(FilterAny, post.ID)
			// Verify attachments do not already exist at destination board.
			for _, p := range posts {
				err := checkAttachment(p, destination)
				if err != nil {
					data.ManageError(err.Error())
					return
				}
			}
			// Copy attachments.
			for _, p := range posts {
				err := copyAttachment(p, destination)
				if err != nil {
					data.ManageError(err.Error())
					return
				}
			}
			// Remove source attachments.
			for _, p := range posts {
				removeSourceAttachment(p)
			}
			// Delete thread page.
			removeThreadPage(post)
			// Update posts.
			source := post.Board
			for _, p := range posts {
				updatePost(p, destination, 0)
			}
			post = db.PostByID(post.ID)
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
					Message:   Get(destination, nil, "Moved thread from %[1]s to %[2]s.", sourceLink, destinationLink),
					Moderated: ModeratedApproved,
				}
				p.SetNameBlock("", "Mod", false, false)
				db.AddPost(p)
				go s.refreshDiskSpace()
			}
			// Rebuild static files.
			wg := &sync.WaitGroup{}
			delta := &atomic.Int32{}
			db.SoftCommit()
			s.rebuildThread(db, wg, delta, post)
			wg.Wait()
			data.Redirect(w, r, fmt.Sprintf("/sriracha/board/mod/%d/%d", destination.ID, post.Thread()))
		} else {
			data.Threads = append(data.Threads, selected)
			moveLabel := Get(data.Board, data.Account, "Move")
			threadLabel := Get(data.Board, data.Account, "Thread")
			data.Message = `<div style="display: flex;flex-wrap: wrap;row-gap: 15px;column-gap: 15px;"><fieldset><legend>` + data.GetHTML("Move to Thread") + `</legend><form method="post"><table class="manageform"><tr><td class="postblock">` + template.HTML(html.EscapeString(threadLabel)) + `</td><td><input type="text" name="thread" size="8"></td></tr>`
			if len(selected) == 1 && post.Parent == 0 {
				data.Message += `<tr><td colspan="2">&nbsp;</td>`
			}
			data.Message += `<tr><td>&nbsp;</td><td align="right"><input type="submit" class="managebutton" style="width: auto;min-width: 50%;" value="` + template.HTML(html.EscapeString(moveLabel)) + `"></td></tr></table></form></fieldset><br><br>`

			if len(selected) == 1 && post.Parent == 0 {
				boardLabel := Get(data.Board, data.Account, "Board")
				data.Message += `<fieldset><legend>` + data.GetHTML("Move to Board") + `</legend><form method="post"><table class="manageform"><tr><td class="postblock">` + template.HTML(html.EscapeString(boardLabel)) + `</td><td><select name="board">`
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

			data.Message += `</div>`
		}
		return
	} else if action == "sp" || action == "usp" {
		post := selected[0]
		var rebuild bool
		if action == "sp" && !post.Spoiler() {
			if s.forbidden(w, data, "post.spoiler") {
				return
			}
			db.SpoilerPost(post.ID, true)
			s.log(db, data.Account, nil, fmt.Sprintf("Spoilered >>/post/%d", post.ID), "")
			rebuild = true
		} else if action == "usp" && post.Spoiler() {
			if s.forbidden(w, data, "post.spoiler") {
				return
			}
			db.SpoilerPost(post.ID, false)
			s.log(db, data.Account, nil, fmt.Sprintf("Unspoilered >>/post/%d", post.ID), "")
			rebuild = true
		}
		if rebuild {
			wg := &sync.WaitGroup{}
			delta := &atomic.Int32{}
			db.SoftCommit()
			s.rebuildThread(db, wg, delta, post)
			wg.Wait()
		}
		data.Redirect(w, r, fmt.Sprintf("/sriracha/board/mod/%d/%d", post.Board.ID, post.Thread()))
		return
	} else if action == "da" {
		post := selected[0]
		if post.File == "" {
			data.ManageError("That post does not have an attachment.")
			return
		} else if post.Message == "" {
			data.ManageError("That post does not have a message. Delete the post instead.")
			return
		}

		s.deletePostAttachment(db, post, true, true)
		s.log(db, data.Account, post.Board, fmt.Sprintf("Deleted attachment from >>/post/%d", post.ID), "")

		wg := &sync.WaitGroup{}
		delta := &atomic.Int32{}
		db.SoftCommit()
		s.rebuildThread(db, wg, delta, post)
		wg.Wait()

		data.Template = "manage_info"
		data.Redirect(w, r, fmt.Sprintf("/sriracha/board/mod/%d/%d", post.Board.ID, post.Thread()))
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
			} else if post.Archived() {
				data.ManageError("Only active posts may be stickied.")
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
			} else if post.Archived() {
				data.ManageError("Only active posts may be locked.")
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
			wg := &sync.WaitGroup{}
			delta := &atomic.Int32{}
			db.SoftCommit()
			s.rebuildThread(db, wg, delta, post)
			wg.Wait()
		}

		data.Template = "manage_info"
		data.Redirect(w, r, fmt.Sprintf("/sriracha/board/mod/%d/%d", post.Board.ID, post.Thread()))
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
		var rebuild []*Post
		var threadIDs []int
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
				moderated := post.Moderated
				s.deletePost(db, post)

				message := fmt.Sprintf("Deleted >>%d", post.ID)
				if post.Parent == 0 {
					switch moderated {
					case ModeratedPruned:
						message = fmt.Sprintf("Deleted pruned thread >>%d", post.ID)
					case ModeratedArchived:
						message = fmt.Sprintf("Deleted archived thread >>%d", post.ID)
					}
				} else {
					switch moderated {
					case ModeratedPruned:
						message = fmt.Sprintf("Deleted pruned post >>%d", post.ID)
					case ModeratedArchived:
						message = fmt.Sprintf("Deleted archived post >>%d", post.ID)
					}
				}
				s.log(db, data.Account, data.Board, message, "")

				rebuild = append(rebuild, post)
				threadID := post.Thread()
				if !slices.Contains(threadIDs, threadID) {
					threadIDs = append(threadIDs, threadID)
				}
			}
		}

		for _, threadID := range threadIDs {
			s.unBumpThread(db, threadID)
		}

		wg := &sync.WaitGroup{}
		delta := &atomic.Int32{}
		db.SoftCommit()
		s.rebuildThreads(db, wg, delta, rebuild)
		wg.Wait()

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
