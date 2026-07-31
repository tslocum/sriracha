package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/gabriel-vasile/mimetype"
)

func (s *Server) loadSettingFormFile(db serverDB, r *http.Request) ([]byte, int, int, error) {
	if r.PostForm == nil {
		const maxMemory = 32 << 20 // 32 MB
		err := r.ParseMultipartForm(maxMemory)
		if err != nil {
			return nil, 0, 0, err
		}
	}
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, 0, 0, nil
	}
	files := r.MultipartForm.File["icon"]
	if len(files) == 0 {
		return nil, 0, 0, nil
	} else if len(files) > 1 {
		return nil, 0, 0, fmt.Errorf("too many files: upload a single file")
	}
	fileHeader := files[0]

	formFile, err := fileHeader.Open()
	if err != nil {
		return nil, 0, 0, err
	}
	defer formFile.Close()

	buf, err := io.ReadAll(formFile)
	if err != nil {
		return nil, 0, 0, err
	}

	mimeType := mimetype.Detect(buf).String()
	if mimeType != "image/png" {
		return nil, 0, 0, fmt.Errorf("invalid icon image: expected image/png, got %s", mimeType)
	}

	imgWidth, imgHeight := s.imageDimensions(bytes.NewReader(buf))
	if imgWidth == 0 || imgHeight == 0 {
		return nil, 0, 0, fmt.Errorf("invalid icon image")
	}
	return buf, imgWidth, imgHeight, nil
}

func (s *Server) serveSetting(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}

	if r.URL.Path == "/sriracha/setting/reset" {
		oldOpt := s.opt

		s.opt.SiteName = defaultServerSiteName
		db.SaveString("sitename", s.opt.SiteName)

		s.opt.SiteDescription = ""
		db.SaveString("sitedescription", s.opt.SiteDescription)

		s.opt.SiteHome = defaultServerSiteHome
		db.SaveString("sitehome", s.opt.SiteHome)

		s.opt.SiteIndex = true
		db.SaveBool("siteindex", s.opt.SiteIndex)

		s.opt.News = NewsDisable
		db.SaveInt("news", int(s.opt.News))

		s.opt.BoardIndex = true
		db.SaveBool("boardindex", s.opt.BoardIndex)

		s.opt.Statistics = false
		db.SaveBool("statistics", s.opt.Statistics)

		s.opt.CAPTCHA = false
		db.SaveBool("captcha", s.opt.CAPTCHA)

		s.opt.OekakiWidth = defaultServerOekakiWidth
		db.SaveInt("oekakiwidth", s.opt.OekakiWidth)

		s.opt.OekakiHeight = defaultServerOekakiHeight
		db.SaveInt("oekakiheight", s.opt.OekakiHeight)

		s.opt.Search = defaultServerSearch
		db.SaveInt("search", s.opt.Search)

		s.opt.Refresh = defaultServerRefresh
		db.SaveInt("refresh", s.opt.Refresh)

		if s.opt.ModQueue != "" {
			os.Remove(filepath.Join(s.config.Root, s.opt.ModQueue+".html"))
		}
		s.opt.ModQueue = ""
		db.SaveString("modqueue", s.opt.ModQueue)

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

		s.opt.Global = nil
		for _, setting := range allGlobalSettings {
			db.SaveBool("global."+setting, false)
		}

		db.ClearBoardCache()
		s.removeInvalidBoardOptions(db)
		s.writeModQueue(db)

		changes := printChanges(oldOpt, s.opt)
		if changes != "" {
			s.log(db, data.Account, nil, "Reset settings", changes)
		}

		s.rebuildAll(db)

		data.Redirect(w, r, "/sriracha/setting")
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

		iconPath := filepath.Join(s.config.Root, "banner", "icon.png")
		if FormString(r, "deleteicon") != "" {
			os.Remove(iconPath)
			s.opt.IconWidth, s.opt.IconHeight = 0, 0
		}
		icon, iconWidth, iconHeight, err := s.loadSettingFormFile(db, r)
		if err != nil {
			data.ManageError(err.Error())
			return
		} else if icon != nil {
			err = os.WriteFile(iconPath, icon, NewFilePermission)
			if err != nil {
				log.Fatalf("failed to write icon %s: %s", iconPath, err.Error())
				return
			}
			s.opt.IconWidth, s.opt.IconHeight = iconWidth, iconHeight
		}

		oldOpt := s.opt

		siteName := FormString(r, "sitename")
		if siteName != "" {
			db.SaveString("sitename", siteName)
			s.opt.SiteName = siteName
		}

		siteDescription := FormString(r, "sitedescription")
		db.SaveString("sitedescription", siteDescription)
		s.opt.SiteDescription = siteDescription

		siteHome := FormString(r, "sitehome")
		if siteHome != "" {
			if !strings.HasSuffix(siteHome, "/") {
				siteHome += "/"
			}
			db.SaveString("sitehome", siteHome)
			s.opt.SiteHome = siteHome
		}

		siteIndex := FormBool(r, "siteindex")
		db.SaveBool("siteindex", siteIndex)
		s.opt.SiteIndex = siteIndex

		news := FormInt(r, "news")
		db.SaveInt("news", news)
		s.opt.News = NewsOption(news)

		boardIndex := FormBool(r, "boardindex")
		db.SaveBool("boardindex", boardIndex)
		s.opt.BoardIndex = boardIndex

		statistics := FormBool(r, "statistics")
		db.SaveBool("statistics", statistics)
		s.opt.Statistics = statistics

		enableCAPTCHA := FormBool(r, "captcha")
		db.SaveBool("captcha", enableCAPTCHA)
		s.opt.CAPTCHA = enableCAPTCHA

		search := FormInt(r, "search")
		db.SaveInt("search", search)
		s.opt.Search = search

		oekakiWidth := FormInt(r, "oekakiwidth")
		db.SaveInt("oekakiwidth", oekakiWidth)
		s.opt.OekakiWidth = oekakiWidth

		oekakiHeight := FormInt(r, "oekakiheight")
		db.SaveInt("oekakiheight", oekakiHeight)
		s.opt.OekakiHeight = oekakiHeight

		refresh := FormInt(r, "refresh")
		db.SaveInt("refresh", refresh)
		s.opt.Refresh = refresh

		modQueue := strings.TrimSuffix(FormString(r, "modqueue"), ".html")
		if modQueue != "" && (!FilePathPattern.MatchString(modQueue) || strings.Contains(modQueue, "..")) {
			data.ManageError("Invalid moderation queue status page file path.")
			return
		} else if s.opt.ModQueue != modQueue && s.opt.ModQueue != "" {
			os.Remove(filepath.Join(s.config.Root, s.opt.ModQueue+".html"))
		}
		db.SaveString("modqueue", modQueue)
		s.opt.ModQueue = modQueue

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

		s.opt.Global = nil
		for _, setting := range allGlobalSettings {
			global := FormBool(r, setting)
			if global {
				s.opt.Global = append(s.opt.Global, setting)
			}
			db.SaveBool("global."+setting, global)
		}

		db.ClearBoardCache()
		s.removeInvalidBoardOptions(db)
		s.writeModQueue(db)

		changes := printChanges(oldOpt, s.opt)
		if changes != "" {
			s.log(db, data.Account, nil, "Updated settings", changes)
		}

		if FormBool(r, "rebuild") {
			s.rebuildAll(db)
		}
	}
	data.Template = "manage_setting"
	data.Extra = SrirachaVersion

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	data.Extra2 = FormatFileSize(int64(m.Sys))

	data.Extra3 = FormatDuration(time.Since(s.config.StartTime))

	data.Extra4 = fmt.Sprintf("%d", s.connCount.Load())
}
