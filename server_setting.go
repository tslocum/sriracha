package sriracha

import (
	"net/http"
)

func (s *Server) serveSetting(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	// TODO restrict access
	if r.URL.Path == "/sriracha/setting/reset" {
		db.SaveString("sitename", defaultServerSiteName)
		s.opt.SiteName = defaultServerSiteName
		db.SaveBool("boardindex", true)
		s.opt.BoardIndex = true
		http.Redirect(w, r, "/sriracha/setting", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		oldOpt := s.opt

		siteName := formString(r, "sitename")
		if siteName != "" {
			db.SaveString("sitename", siteName)
			s.opt.SiteName = siteName
		}

		siteHome := formString(r, "sitehome")
		if siteHome != "" {
			db.SaveString("sitehome", siteHome)
			s.opt.SiteHome = siteHome
		}

		boardIndex := formBool(r, "boardindex")
		db.SaveBool("boardindex", boardIndex)
		s.opt.BoardIndex = boardIndex

		changes := printChanges(oldOpt, s.opt)
		if changes != "" {
			db.log(data.Account, nil, "Updated settings", changes)
		}

		for _, b := range db.allBoards() {
			s.rebuildBoard(db, b)
		}
	}
	data.Template = "manage_setting"
}
