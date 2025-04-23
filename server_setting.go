package sriracha

import (
	"net/http"
)

func (s *Server) serveSetting(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}

	if r.URL.Path == "/sriracha/setting/reset" {
		oldOpt := s.opt

		db.SaveString("sitename", defaultServerSiteName)
		s.opt.SiteName = defaultServerSiteName

		db.SaveString("sitehome", defaultServerSiteHome)
		s.opt.SiteHome = defaultServerSiteHome

		db.SaveBool("boardindex", true)
		s.opt.BoardIndex = true

		db.SaveBool("captcha", false)
		s.opt.CAPTCHA = false

		changes := printChanges(oldOpt, s.opt)
		if changes != "" {
			db.log(data.Account, nil, "Reset settings", changes)
		}

		for _, b := range db.allBoards() {
			s.rebuildBoard(db, b)
		}

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

		enableCAPTCHA := formBool(r, "captcha")
		db.SaveBool("captcha", enableCAPTCHA)
		s.opt.CAPTCHA = enableCAPTCHA

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
