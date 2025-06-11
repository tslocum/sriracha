package sriracha

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
)

func (s *Server) serveSetting(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}

	if r.URL.Path == "/sriracha/setting/reset" {
		oldOpt := s.opt

		s.opt.SiteName = defaultServerSiteName
		db.SaveString("sitename", s.opt.SiteName)

		s.opt.SiteHome = defaultServerSiteHome
		db.SaveString("sitehome", s.opt.SiteHome)

		s.opt.News = NewsDisable
		db.SaveInt("news", int(s.opt.News))

		s.opt.BoardIndex = true
		db.SaveBool("boardindex", s.opt.BoardIndex)

		s.opt.CAPTCHA = false
		db.SaveBool("captcha", s.opt.CAPTCHA)

		s.opt.OekakiWidth = defaultServerOekakiWidth
		db.SaveInt("oekakiwidth", s.opt.OekakiWidth)

		s.opt.OekakiHeight = defaultServerOekakiHeight
		db.SaveInt("oekakiheight", s.opt.OekakiHeight)

		s.opt.Refresh = defaultServerRefresh
		db.SaveInt("refresh", s.opt.Refresh)

		s.opt.Overboard = ""
		db.SaveString("overboard", s.opt.Overboard)

		s.opt.OverboardType = TypeImageboard
		db.SaveInt("overboardtype", int(s.opt.OverboardType))

		s.opt.OverboardThreads = defaultBoardThreads
		db.SaveInt("overboardthreads", s.opt.OverboardThreads)

		s.opt.OverboardReplies = defaultBoardReplies
		db.SaveInt("overboardreplies", s.opt.OverboardReplies)

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

		s.rebuildNews(db)

		http.Redirect(w, r, "/sriracha/setting", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		overboard := formString(r, "overboard")
		if overboard != "" && overboard != "/" {
			if !alphaNumericAndSymbols.MatchString(overboard) {
				data.ManageError("Invalid overboard directory.")
				return
			}
		}

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

		db.SaveString("overboard", overboard)
		s.opt.Overboard = overboard

		overboardType := formRange(r, "overboardtype", TypeImageboard, TypeForum)
		db.SaveInt("overboardtype", int(overboardType))
		s.opt.OverboardType = overboardType

		overboardThreads := formInt(r, "overboardthreads")
		db.SaveInt("overboardthreads", overboardThreads)
		s.opt.OverboardThreads = overboardThreads

		overboardReplies := formInt(r, "overboardreplies")
		db.SaveInt("overboardreplies", overboardReplies)
		s.opt.OverboardReplies = overboardReplies

		if overboard != "" && overboard != "/" {
			os.Mkdir(filepath.Join(s.config.Root, overboard), newDirPermission)
		}

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

		s.rebuildAll(db)
	}
	data.Template = "manage_setting"
	data.Extra = SrirachaVersion
	if SrirachaVersion != "DEV" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision := setting.Value
				if len(revision) > 10 {
					revision = revision[:10]
				}
				data.Extra += "-" + revision
				return
			}
		}
	}
}
