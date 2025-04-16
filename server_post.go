package sriracha

import (
	"fmt"
	"log"
	"net/http"
)

func (s *Server) servePost(db *Database, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid request", http.StatusInternalServerError)
		return
	}

	boardDir := formString(r, "board")
	if boardDir == "" {
		data := s.buildData(db, w, r)
		data.Error("no board was specified")
		data.execute(w)
		return
	}
	b, err := db.boardByDir(boardDir)
	if err != nil {
		log.Fatal(err)
	} else if b == nil {
		data := s.buildData(db, w, r)
		data.Error("no board was specified")
		data.execute(w)
	}

	post := &Post{}
	err = post.loadForm(r, b, s.config.Root)
	if err != nil {
		data := s.buildData(db, w, r)
		data.Error(err.Error())
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

	err = db.addPost(b, post)
	if err != nil {
		log.Fatal(err)
	}

	s.writeThread(post)

	redir := fmt.Sprintf("/res/%d.html#%d", post.ThreadID(), post.ID)
	http.Redirect(w, r, redir, http.StatusFound)
}
