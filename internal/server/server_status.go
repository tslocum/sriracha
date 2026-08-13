package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveStatus(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		scanIDs := func(formKey string) []int {
			formValue := FormString(r, formKey)
			if formValue == "" {
				return nil
			}
			var allIDs []int
			if strings.ContainsRune(formValue, ',') {
				split := strings.Split(formValue, ",")
				for _, splitStr := range split {
					id, err := strconv.Atoi(splitStr)
					if err != nil || slices.Contains(allIDs, id) {
						continue
					}
					allIDs = append(allIDs, id)
				}
			} else {
				id := FormInt(r, formKey)
				allIDs = append(allIDs, id)
			}
			return allIDs
		}

		var action string
		if FormString(r, "approve") != "" {
			action = "approve"
		} else if FormString(r, "archive") != "" {
			action = "archive"
		} else {
			data.ManageError("Unknown moderation action.")
			return
		}
		ids := scanIDs(action)
		if len(ids) == 0 {
			data.ManageError("No post selected.")
			return
		}
		log.Println(action, ids)

		wg := &sync.WaitGroup{}
		delta := &atomic.Uint32{}
		for _, postID := range ids {
			post := db.PostByID(postID)
			if post == nil {
				continue
			}

			if action == "approve" {
				if post.Archived() {
					continue
				}
				rebuild := post.Moderated == ModeratedHidden

				db.ModeratePost(post.ID, ModeratedApproved)
				db.DeleteReports(post)

				if !rebuild {
					continue
				}
				db.AddPostBacklinks(post)
				db.BumpThread(post.Thread(), time.Now().Unix())
			} else { // action = archive
				if post.Moderated == ModeratedArchived {
					continue
				}
				for _, p := range db.AllPostsInThread(FilterAny, post.ID) {
					db.ModeratePost(p.ID, ModeratedArchived)
					db.DeleteReports(p)
				}
				if post.Stickied {
					db.StickyPost(post.ID, false)
				}
				if post.Locked {
					db.LockPost(post.ID, false)
				}
				db.BumpThread(post.Thread(), time.Now().Unix())
			}
		}
		db.SoftCommit()
		var posts []*Post
		for _, postID := range ids {
			post := db.PostByID(postID)
			if post == nil {
				continue
			}
			posts = append(posts, post)
		}
		s.rebuildThreads(db, wg, delta, posts)
		wg.Wait()

		if action == "approve" && s.opt.Notifications {
			for _, postID := range ids {
				post := db.PostByID(postID)
				if post == nil || post.Archived() {
					continue
				}
				s.queueNotifications(db, post)
			}
		}

		data.Redirect(w, r, "/sriracha/")
		return
	}

	buf := &bytes.Buffer{}
	data.Template = "manage_status"

	// Allow super-administrators to verify remote address resolution.
	if r.URL.Query().Has("remoteAddress") {
		if data.forbidden(w, RoleSuperAdmin) {
			return
		}
		data.Info = "Remote address: " + s.requestIP(r)
		return
	}

	// Allow super-administrators to rebuild post nameblocks.
	if r.URL.Query().Has("rebuildNameblocks") {
		if data.forbidden(w, RoleSuperAdmin) {
			return
		}
		s.rebuildNameblocks(db)
		wg := &sync.WaitGroup{}
		delta := &atomic.Uint32{}
		db.SoftCommit()
		for _, b := range db.AllBoards() {
			s.rebuildBoard(db, wg, delta, b)
		}
		wg.Wait()
		data.Info = "Rebuilt nameblocks"
		return
	}

	// Allow super-administrators to rebuild reflinks.
	if r.URL.Query().Has("rebuildReflinks") {
		if data.forbidden(w, RoleSuperAdmin) {
			return
		}
		data.Template = "manage_info"
		for _, b := range db.AllBoards() {
			for _, thread := range db.AllThreads(FilterAny, b) {
				for _, p := range db.AllPostsInThread(FilterAny, thread[0]) {
					resPattern := regexp.MustCompile(`<a href="[^"]*res\/([0-9]+).html#([0-9]+)" class="([A-Aa-z]+)">&gt;&gt;([0-9]+)(\(OP\))?</a>`)
					oldMessage := p.Message
					p.Message = resPattern.ReplaceAllStringFunc(p.Message, func(s string) string {
						match := resPattern.FindStringSubmatch(s)
						threadID := ParseInt(match[1])
						postID := ParseInt(match[2])
						var extra string
						if postID == threadID {
							extra = "(OP)"
						}
						return fmt.Sprintf(`<a href="%sres/%d.html#%d" class="%s">&gt;&gt;%d%s</a>`, b.Path(), threadID, postID, match[3], postID, extra)
					})
					if p.Message != oldMessage {
						db.UpdatePostMessage(p.ID, p.Message)
					}
				}
			}
		}
		db.SoftCommit()
		s.rebuildAll(db)
		data.Info = "Rebuilt reflinks"
		return
	}

	// Allow super-administrators to scan for unexpected files.
	if r.URL.Query().Has("scanFiles") {
		if data.forbidden(w, RoleSuperAdmin) {
			return
		}
		data.Template = "manage_info"
		data.Message = `<h2 class="managetitle">Scan Files</h2>`
		var scanned int
		var found []string
		checkBoardDir := func(b *Board, dir string) {
			boardDir := filepath.Join(s.config.Root, b.Dir)
			checkDir := filepath.Join(boardDir, dir)
			err := filepath.WalkDir(checkDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				} else if d.IsDir() {
					return nil
				}
				scanned++
				if filepath.Dir(path) != checkDir {
					found = append(found, path)
					return nil
				}
				fieldName := "file"
				if dir == "thumb" {
					fieldName = "thumb"
				}
				post := db.PostByField(b, fieldName, filepath.Base(path))
				if post == nil {
					found = append(found, path)
				}
				return nil
			})
			if err != nil {
				log.Fatalf("failed to scan directory %s: %s", checkDir, err)
			}
		}
		for _, b := range db.AllBoards() {
			checkBoardDir(b, "src")
			checkBoardDir(b, "thumb")
			resDir := filepath.Join(s.config.Root, b.Dir, "res")
			err := filepath.WalkDir(resDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				} else if d.IsDir() {
					return nil
				}
				scanned++
				if filepath.Dir(path) != resDir || !strings.HasSuffix(path, ".html") {
					found = append(found, path)
					return nil
				}
				id, err := strconv.Atoi(strings.TrimSuffix(filepath.Base(path), ".html"))
				if err != nil || id <= 0 {
					found = append(found, path)
					return nil
				}
				post := db.PostByID(id)
				if post == nil || post.Parent != 0 {
					found = append(found, path)
				}
				return nil
			})
			if err != nil {
				log.Fatalf("failed to scan directory %s: %s", resDir, err)
			}
		}
		data.Message += template.HTML(fmt.Sprintf("&nbsp; Scanned %d files", scanned))
		if len(found) == 0 {
			data.Message += template.HTML(". All files validated.")
		} else {
			data.Message += template.HTML(fmt.Sprintf(" and found %d unexpected files:<ul>", len(found)))
			for _, filePath := range found {
				relativePath := strings.TrimPrefix(filePath, s.config.Root)
				data.Message += template.HTML(fmt.Sprintf(`<li><a href="%s">%s</a></li>`, relativePath, relativePath))
			}
			data.Message += template.HTML("</ul><fieldset><legend>Remove unexpected files</legend><textarea style=\"width: 500px;height: 200px;\">")
			for i, filePath := range found {
				if i != 0 {
					data.Message += template.HTML("\n")
				}
				relativePath := strings.TrimPrefix(filePath, s.config.Root)
				data.Message += template.HTML(fmt.Sprintf("rm '%s'", relativePath[1:]))
			}
			data.Message += template.HTML("</textarea></fieldset><br>")
		}
		return
	}

	reports := db.AllReports()
	for i, report := range reports {
		if i > 0 {
			buf.WriteString("<hr>\n")
		}

		d := s.buildData(db, w, r)
		d.Template = "manage_status_item"
		d.Board = report.Post.Board
		d.Post = report.Post
		d.Threads = [][]*Post{{report.Post}}
		d.Manage.Report = report
		d.execute(buf)
	}
	if len(reports) > 1 {
		var ids []byte
		for i := range reports {
			if i != 0 {
				ids = append(ids, ',')
			}
			ids = append(ids, []byte(strconv.Itoa(reports[i].Post.ID))...)
		}
		buf.WriteString(fmt.Sprintf(`<hr><form method="post" action="/sriracha/" style="display: inline-block;"><input type="hidden" name="approve" value="%s"><input type="submit" value="%s"></form>`, ids, Get(nil, data.Account, "Approve all")))
	}
	data.Message = template.HTML(buf.String())

	buf.Reset()
	pending := db.PendingPosts()
	for i, post := range pending {
		if i > 0 {
			buf.WriteString("<hr>\n")
		}

		d := s.buildData(db, w, r)
		d.Template = "manage_status_item"
		d.Board = post.Board
		d.Post = post
		d.Threads = [][]*Post{{post}}
		d.execute(buf)
	}
	if len(pending) > 1 {
		var ids []byte
		for i := range pending {
			if i != 0 {
				ids = append(ids, ',')
			}
			ids = append(ids, []byte(strconv.Itoa(pending[i].ID))...)
		}
		buf.WriteString(fmt.Sprintf(`<hr><form method="post" action="/sriracha/" style="display: inline-block;"><input type="hidden" name="approve" value="%s"><input type="submit" value="%s"></form>`, ids, Get(nil, data.Account, "Approve all")))
	}
	data.Message2 = template.HTML(buf.String())

	buf.Reset()
	pruned := db.PrunedThreads()
	for i, threadID := range pruned {
		if i > 0 {
			buf.WriteString("<hr>\n")
		}

		posts := db.AllPostsInThread(FilterAny, threadID)

		d := s.buildData(db, w, r)
		d.Template = "imgboard_post"
		d.Board = posts[0].Board
		d.Post = posts[0]
		d.Threads = [][]*Post{posts}
		d.ReplyMode = posts[0].ID
		d.execute(buf)
	}
	if len(pruned) > 1 {
		var ids []byte
		for i := range pruned {
			if i != 0 {
				ids = append(ids, ',')
			}
			ids = append(ids, []byte(strconv.Itoa(pruned[i]))...)
		}
		buf.WriteString(fmt.Sprintf(`<hr><form method="post" action="/sriracha/" style="display: inline-block;"><input type="hidden" name="archive" value="%s"><input type="submit" value="%s"></form>`, ids, Get(nil, data.Account, "Archive all")))
	}
	data.Message3 = template.HTML(buf.String())

	total := len(reports) + len(pending) + len(pruned)
	if total > 0 {
		data.Extra3 = data.GetN("%d pending moderation request", "%d pending moderation requests", total)
	}
}
