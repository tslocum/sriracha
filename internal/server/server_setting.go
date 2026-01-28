package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) serveSetting(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
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

		s.opt.OverboardThreads = DefaultBoardThreads
		db.SaveInt("overboardthreads", s.opt.OverboardThreads)

		s.opt.OverboardReplies = DefaultBoardReplies
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
			s.log(db, data.Account, nil, "Reset settings", changes)
		}

		for _, b := range db.AllBoards() {
			s.rebuildBoard(db, b)
		}

		s.rebuildNews(db)

		http.Redirect(w, r, "/sriracha/setting", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		overboard := FormString(r, "overboard")
		if overboard != "" && overboard != "/" {
			if !AlphaNumericAndSymbols.MatchString(overboard) {
				data.ManageError("Invalid overboard directory.")
				return
			}
		}

		oldOpt := s.opt

		siteName := FormString(r, "sitename")
		if siteName != "" {
			db.SaveString("sitename", siteName)
			s.opt.SiteName = siteName
		}

		siteHome := FormString(r, "sitehome")
		if siteHome != "" {
			if !strings.HasSuffix(siteHome, "/") {
				siteHome += "/"
			}
			db.SaveString("sitehome", siteHome)
			s.opt.SiteHome = siteHome
		}

		news := FormInt(r, "news")
		db.SaveInt("news", news)
		s.opt.News = NewsOption(news)

		boardIndex := FormBool(r, "boardindex")
		db.SaveBool("boardindex", boardIndex)
		s.opt.BoardIndex = boardIndex

		enableCAPTCHA := FormBool(r, "captcha")
		db.SaveBool("captcha", enableCAPTCHA)
		s.opt.CAPTCHA = enableCAPTCHA

		oekakiWidth := FormInt(r, "oekakiwidth")
		db.SaveInt("oekakiwidth", oekakiWidth)
		s.opt.OekakiWidth = oekakiWidth

		oekakiHeight := FormInt(r, "oekakiheight")
		db.SaveInt("oekakiheight", oekakiHeight)
		s.opt.OekakiHeight = oekakiHeight

		refresh := FormInt(r, "refresh")
		db.SaveInt("refresh", refresh)
		s.opt.Refresh = refresh

		db.SaveString("overboard", overboard)
		s.opt.Overboard = overboard

		overboardType := FormRange(r, "overboardtype", TypeImageboard, TypeForum)
		db.SaveInt("overboardtype", int(overboardType))
		s.opt.OverboardType = overboardType

		overboardThreads := FormInt(r, "overboardthreads")
		db.SaveInt("overboardthreads", overboardThreads)
		s.opt.OverboardThreads = overboardThreads

		overboardReplies := FormInt(r, "overboardreplies")
		db.SaveInt("overboardreplies", overboardReplies)
		s.opt.OverboardReplies = overboardReplies

		if overboard != "" && overboard != "/" {
			os.Mkdir(filepath.Join(s.config.Root, overboard), NewDirPermission)
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
			s.log(db, data.Account, nil, "Updated settings", changes)
		}

		s.rebuildAll(db)
	}
	data.Template = "manage_setting"
	data.Extra = SrirachaVersion

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	data.Extra2 = FormatFileSize(int64(m.Sys))

	data.Extra3 = fmt.Sprintf("%s", time.Since(s.config.StartTime).Round(time.Second))
}
