package sriracha

import (
	"net/http"
	"sort"
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

		db.SaveInt("news", int(NewsDisable))
		s.opt.News = NewsDisable

		db.SaveBool("boardindex", true)
		s.opt.BoardIndex = true

		db.SaveBool("captcha", false)
		s.opt.CAPTCHA = false

		db.SaveInt("oekakiwidth", defaultServerOekakiWidth)
		s.opt.OekakiWidth = defaultServerOekakiWidth

		db.SaveInt("oekakiheight", defaultServerOekakiHeight)
		s.opt.OekakiHeight = defaultServerOekakiHeight

		db.SaveInt("refresh", defaultServerRefresh)
		s.opt.Refresh = defaultServerRefresh

		s.opt.Embeds = nil
		var embeds []string
		for _, info := range defaultServerEmbeds {
			embedName, embedURL := info[0], info[1]
			s.opt.Embeds = append(s.opt.Embeds, info)
			embeds = append(embeds, embedName+" "+embedURL)
		}
		db.SaveMultiString("embeds", embeds)

		changes := printChanges(oldOpt, s.opt)
		if changes != "" {
			db.log(data.Account, nil, "Reset settings", changes)
		}

		for _, b := range db.AllBoards() {
			s.rebuildBoard(db, b)
		}

		s.rebuildAllNews(db)

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

		news := formInt(r, "news")
		db.SaveInt("news", news)
		s.opt.News = NewsOption(news)

		boardIndex := formBool(r, "boardindex")
		db.SaveBool("boardindex", boardIndex)
		s.opt.BoardIndex = boardIndex

		enableCAPTCHA := formBool(r, "captcha")
		db.SaveBool("captcha", enableCAPTCHA)
		s.opt.CAPTCHA = enableCAPTCHA

		oekakiWidth := formInt(r, "oekakiwidth")
		db.SaveInt("oekakiwidth", oekakiWidth)
		s.opt.OekakiWidth = oekakiWidth

		oekakiHeight := formInt(r, "oekakiheight")
		db.SaveInt("oekakiheight", oekakiHeight)
		s.opt.OekakiHeight = oekakiHeight

		refresh := formInt(r, "refresh")
		db.SaveInt("refresh", refresh)
		s.opt.Refresh = refresh

		s.opt.Embeds = nil
		r.ParseForm()
		var embedNames []string
		for name := range r.Form {
			if strings.HasPrefix(name, "embeds") {
				embedNames = append(embedNames, name)
			}
		}
		sort.Strings(embedNames)
		var embeds []string
		for _, name := range embedNames {
			for _, vv := range r.Form[name] {
				value := strings.TrimSpace(vv)
				if value == "" {
					continue
				}
				split := strings.SplitN(value, " ", 2)
				if len(split) != 2 || (!strings.HasPrefix(split[1], "http://") && !strings.HasPrefix(split[1], "https://")) || !strings.Contains(split[1], "SRIRACHA_EMBED") {
					continue
				}
				s.opt.Embeds = append(s.opt.Embeds, [2]string{split[0], split[1]})
				embeds = append(embeds, vv)
			}
		}
		db.SaveMultiString("embeds", embeds)

		changes := printChanges(oldOpt, s.opt)
		if changes != "" {
			db.log(data.Account, nil, "Updated settings", changes)
		}

		for _, b := range db.AllBoards() {
			s.rebuildBoard(db, b)
		}

		s.rebuildAllNews(db)
	}
	data.Template = "manage_setting"
	data.Extra = SrirachaVersion
}
