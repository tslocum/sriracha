package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveStatus(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		approve := FormInt(r, "approve")
		if approve > 0 {
			boardID := FormInt(r, "board")
			if boardID > 0 {
				b := db.BoardByID(boardID)
				if b != nil {
					post := db.PostByID(approve)
					if post != nil {
						rebuild := post.Moderated == ModeratedHidden

						db.ModeratePost(post.ID, ModeratedApproved)
						db.DeleteReports(post)

						if rebuild {
							db.BumpThread(post.Thread(), time.Now().Unix())
							s.rebuildThread(db, post)
							s.queueNotifications(db, post)
						}
					}
				}
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
	}

	// Allow super-administrators to rebuild post nameblocks.
	if r.URL.Query().Has("rebuildNameblocks") {
		if data.forbidden(w, RoleSuperAdmin) {
			return
		}
		for _, b := range db.AllBoards() {
			for _, threadInfo := range db.AllThreads(b, false) {
				for _, p := range db.AllPostsInThread(threadInfo[0], false) {
					var capcode string
					if strings.Contains(p.NameBlock, `<span style="color: red`) {
						capcode = "Mod"
					} else if strings.Contains(p.NameBlock, `<span style="color: purple`) {
						capcode = "Admin"
					}
					p.SetNameBlock(p.Board.DefaultName, capcode, s.opt.Identifiers)

					db.UpdatePostNameblock(p.ID, p.NameBlock)
				}
			}
			s.rebuildBoard(db, b)
		}
		data.Info = "Rebuilt nameblocks"
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
			data.Message += template.HTML(" and only found expected files.")
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
	data.Message2 = template.HTML(buf.String())
}
