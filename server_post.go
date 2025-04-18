package sriracha

import (
	"fmt"
	"log"
	"net/http"
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
		data.Error("no board was specified")
		data.execute(w)
		return
	}

	now := time.Now().Unix()
	post := &Post{
		Timestamp: now,
		Bumped:    now,
	}
	err := post.loadForm(r, b, s.config.Root)
	if err != nil {
		data := s.buildData(db, w, r)
		data.Error(err.Error())
		data.execute(w)
		return
	}

	if post.Parent != 0 {
		parent := db.postByID(b, post.Parent)
		if parent == nil || parent.Parent != 0 {
			data := s.buildData(db, w, r)
			data.Error("invalid post parent")
			data.execute(w)
			return
		}
	}

	if post.Message == "" && post.File == "" {
		data := s.buildData(db, w, r)
		data.Error("Please upload a file and/or enter a message.")
		data.execute(w)
		return
	}

	for _, postHandler := range allPluginPostHandlers {
		err := postHandler(db, post)
		if err != nil {
			// TODO cleanup uploaded file
			data := s.buildData(db, w, r)
			data.Error(err.Error())
			data.execute(w)
			log.Printf("warning: plugin failed to handle post event: %s", err.Error())
			return
		}
	}

	post.Message = strings.ReplaceAll(post.Message, "\n", "<br>\n")

	db.addPost(b, post)

	if post.Parent != 0 && strings.ToLower(post.Email) != "sage" {
		// TODO check reply limit
		db.bumpThread(post.Parent, now)
	}

	s.rebuildThread(db, b, post)

	redir := fmt.Sprintf("%sres/%d.html#%d", b.Path(), post.ThreadID(), post.ID)
	http.Redirect(w, r, redir, http.StatusFound)
}
