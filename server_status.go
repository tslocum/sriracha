package sriracha

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"
	"time"
)

func (s *Server) serveStatus(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		approve := formInt(r, "approve")
		if approve > 0 {
			boardID := formInt(r, "board")
			if boardID > 0 {
				b := db.BoardByID(boardID)
				if b != nil {
					post := db.PostByID(approve)
					if post != nil {
						rebuild := post.Moderated == ModeratedHidden

						db.moderatePost(post.ID, ModeratedApproved)
						db.deleteReports(post)

						if rebuild {
							db.bumpThread(post.Thread(), time.Now().Unix())
							s.rebuildThread(db, post)
						}
					}
				}
			}
		}

		http.Redirect(w, r, "/sriracha/", http.StatusFound)
		return
	}

	buf := &bytes.Buffer{}
	data.Template = "manage_status"

	// Allow super-administrators to verify remote address resolution.
	if r.URL.Query().Has("remoteAddress") {
		if data.forbidden(w, RoleSuperAdmin) {
			return
		}
		data.Info = "Remote address: " + requestIP(r)
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
					p.setNameBlock(p.Board.DefaultName, capcode)

					db.updatePostNameblock(p.ID, p.NameBlock)
				}
			}
			s.rebuildBoard(db, b)
		}
		data.Info = "Rebuilt nameblocks"
	}

	reports := db.allReports()
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
	pending := db.pendingPosts()
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
