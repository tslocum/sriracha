package sriracha

import (
	"net/http"
)

func (s *Server) serveSetting(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	// TODO restrict access
	if r.URL.Path == "/sriracha/setting/reset" {
		db.SaveString("sitename", defaultServerSiteName)
		s.opt.SiteName = defaultServerSiteName
		http.Redirect(w, r, "/sriracha/setting", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		siteName := formString(r, "sitename")
		if siteName != "" {
			db.SaveString("sitename", siteName)
			s.opt.SiteName = siteName
		}
	}
	data.Template = "manage_setting"
}
