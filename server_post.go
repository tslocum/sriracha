package sriracha

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func (s *Server) servePost(db *Database, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid request", http.StatusInternalServerError)
		return
	}

	boardDir := formString(r, "board")
	b := db.boardByDir(boardDir)
	if b == nil {
		data := s.buildData(db, w, r)
		data.ManageError("no board was specified")
		data.execute(w)
		return
	}

	now := time.Now().Unix()
	post := &Post{
		Timestamp: now,
		Bumped:    now,
		Moderated: 1,
	}
	err := post.loadForm(r, b, s.config.Root)
	if err != nil {
		data := s.buildData(db, w, r)
		data.ManageError(err.Error())
		data.execute(w)
		return
	}

	if post.Parent != 0 {
		parent := db.postByID(b, post.Parent)
		if parent == nil || parent.Parent != 0 {
			data := s.buildData(db, w, r)
			data.ManageError("invalid post parent")
			data.execute(w)
			return
		}
	}

	ip := r.RemoteAddr
	if ip != "" {
		post.IP = hashIP(ip)
	}

	password := formString(r, "password")
	if password != "" {
		post.Password = hashData(password)
	}

	for _, keyword := range db.allKeywords() {
		rgxp, err := regexp.Compile(keyword.Text)
		if err != nil {
			log.Fatalf("failed to compile regexp %s: %s", keyword.Text, err)
		}
		if rgxp.MatchString(post.Name) || rgxp.MatchString(post.Email) || rgxp.MatchString(post.Subject) || rgxp.MatchString(post.Message) {
			var action string
			var banExpire int64
			switch keyword.Action {
			case "hide":
				action = "hide"
			case "report":
				action = "report"
			case "delete":
				action = "delete"
			case "ban1h":
				action = "ban"
				banExpire = time.Now().Add(1 * time.Hour).Unix()
			case "ban1d":
				action = "ban"
				banExpire = time.Now().Add(24 * time.Hour).Unix()
			case "ban2d":
				action = "ban"
				banExpire = time.Now().Add(2 * 24 * time.Hour).Unix()
			case "ban1w":
				action = "ban"
				banExpire = time.Now().Add(7 * 24 * time.Hour).Unix()
			case "ban2w":
				action = "ban"
				banExpire = time.Now().Add(14 * 24 * time.Hour).Unix()
			case "ban1m":
				action = "ban"
				banExpire = time.Now().Add(28 * 24 * time.Hour).Unix()
			case "ban0":
				action = "ban"
			default:
				log.Fatalf("unknown keyword action: %s", keyword.Action)
			}

			switch action {
			case "hide":
				post.Moderated = 0
			case "report":
				// TODO add report
			case "ban":
				_ = banExpire
				// TODO
			}

			if action == "delete" || action == "ban" {
				s.deletePostFiles(b, post)
				return
			}
		}
	}

	for _, postHandler := range allPluginPostHandlers {
		err := postHandler(db, post)
		if err != nil {
			// TODO cleanup uploaded file
			data := s.buildData(db, w, r)
			data.ManageError(err.Error())
			data.execute(w)
			log.Printf("warning: plugin failed to handle post event: %s", err.Error())
			return
		}
	}

	if strings.TrimSpace(post.Message) == "" && post.File == "" {
		data := s.buildData(db, w, r)
		data.ManageError("Please upload a file and/or enter a message.")
		data.execute(w)
		return
	}

	post.Message = strings.ReplaceAll(post.Message, "\n", "<br>\n")

	if b.Approval == ApprovalAll || (b.Approval == ApprovalFile && post.File != "") {
		post.Moderated = 0
	}

	db.addPost(b, post)

	if post.Parent != 0 && strings.ToLower(post.Email) != "sage" {
		// TODO check reply limit
		db.bumpThread(post.Parent, now)
	}

	s.rebuildThread(db, b, post)

	redir := fmt.Sprintf("%sres/%d.html#%d", b.Path(), post.Thread(), post.ID)
	http.Redirect(w, r, redir, http.StatusFound)
}
