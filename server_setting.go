package sriracha

import (
	"net/http"
	"strings"
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

		clear(s.opt.Embeds)
		var embeds []string
		for embedName, embedURL := range defaultServerEmbeds {
			s.opt.Embeds[embedName] = embedURL
			embeds = append(embeds, embedName+" "+embedURL)
		}
		db.SaveMultiString("embeds", embeds)

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

		clear(s.opt.Embeds)
		r.ParseForm()
		var embeds []string
		for name, v := range r.Form {
			if !strings.HasPrefix(name, "embeds") {
				continue
			}
			for _, vv := range v {
				value := strings.TrimSpace(vv)
				if value == "" {
					continue
				}
				split := strings.SplitN(value, " ", 2)
				if len(split) != 2 || (!strings.HasPrefix(split[1], "http://") && !strings.HasPrefix(split[1], "https://")) || !strings.Contains(split[1], "SRIRACHA_EMBED") {
					continue
				}
				s.opt.Embeds[split[0]] = split[1]
				embeds = append(embeds, vv)
			}
		}
		db.SaveMultiString("embeds", embeds)

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
