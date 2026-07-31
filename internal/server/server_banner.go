package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/gabriel-vasile/mimetype"
)

func (s *Server) loadGlobalBannerSettings(db serverDB, b *Banner) {
	var cachedBanner *Banner
	var fetchedBanner bool
	firstBanner := func() *Banner {
		if fetchedBanner {
			return cachedBanner
		}
		allBanners := db.AllBanners()
		if len(allBanners) > 0 {
			cachedBanner = allBanners[0]
		}
		fetchedBanner = true
		return cachedBanner
	}
	if slices.Contains(s.opt.Global, "banner.boards") {
		first := firstBanner()
		if first != nil {
			b.Boards = first.Boards
		} else {
			b.Boards = nil
		}
	}
	if slices.Contains(s.opt.Global, "banner.overboard") {
		first := firstBanner()
		if first != nil {
			b.Overboard = first.Overboard
		} else {
			b.Overboard = false
		}
	}
	if slices.Contains(s.opt.Global, "banner.news") {
		first := firstBanner()
		if first != nil {
			b.News = first.News
		} else {
			b.News = false
		}
	}
	if slices.Contains(s.opt.Global, "banner.pages") {
		first := firstBanner()
		if first != nil {
			b.Pages = first.Pages
		} else {
			b.Pages = false
		}
	}
}

func (s *Server) loadBannerFormFile(db serverDB, r *http.Request, b *Banner) ([]byte, error) {
	if r.PostForm == nil {
		const maxMemory = 32 << 20 // 32 MB
		err := r.ParseMultipartForm(maxMemory)
		if err != nil {
			return nil, err
		}
	}
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, nil
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		return nil, nil
	} else if len(files) > 1 {
		return nil, fmt.Errorf("too many files: upload a single file")
	}
	fileHeader := files[0]

	formFile, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer formFile.Close()

	buf, err := io.ReadAll(formFile)
	if err != nil {
		return nil, err
	}

	if b.Name == "" {
		b.Name = strings.TrimSpace(fileHeader.Filename)
		if b.Name == "" {
			b.Name = fmt.Sprintf("%d", time.Now().Unix())
		}
	}

	mimeType := mimetype.Detect(buf).String()
	ext := MIMEToExt(mimeType)
	if ext == "" {
		return nil, fmt.Errorf("unknown file type: upload a JPG, PNG or GIF image")
	}
	fileExt := "." + ext
	if !strings.HasSuffix(b.Name, fileExt) {
		b.Name = strings.TrimSuffix(b.Name, filepath.Ext(b.Name)) + fileExt
	}

	imgWidth, imgHeight := s.imageDimensions(bytes.NewReader(buf))
	if imgWidth == 0 || imgHeight == 0 {
		return nil, fmt.Errorf("invalid image")
	}
	b.Width, b.Height = imgWidth, imgHeight

	return buf, nil
}

func (s *Server) loadBannerForm(db serverDB, r *http.Request, b *Banner) {
	b.Name = FormString(r, "name")
	b.Overboard = FormBool(r, "overboard")
	b.News = FormBool(r, "news")
	b.Pages = FormBool(r, "pages")

	b.Boards = nil
	boards := r.Form["boards"]
	for _, board := range boards {
		boardID, err := strconv.Atoi(board)
		if err != nil || boardID <= 0 {
			continue
		}
		board := db.BoardByID(boardID)
		if board == nil {
			continue
		}
		b.Boards = append(b.Boards, board)
	}
}

func (s *Server) saveGlobalBannerSettings(db serverDB, b *Banner) {
	var haveGlobal bool
	for _, setting := range s.opt.Global {
		if strings.HasPrefix(setting, "banner.") {
			haveGlobal = true
			break
		}
	}
	if !haveGlobal {
		return
	}

	allBanners := db.AllBanners()
	var modified bool
	for _, banner := range allBanners {
		if banner.ID == b.ID {
			continue
		}
		modified = false
		if slices.Contains(s.opt.Global, "banner.boards") && !slices.Equal(banner.Boards, b.Boards) {
			banner.Boards = b.Boards
			modified = true
		}
		if slices.Contains(s.opt.Global, "banner.overboard") && banner.Overboard != b.Overboard {
			banner.Overboard = b.Overboard
			modified = true
		}
		if slices.Contains(s.opt.Global, "banner.news") && banner.News != b.News {
			banner.News = b.News
			modified = true
		}
		if slices.Contains(s.opt.Global, "banner.pages") && banner.Pages != b.Pages {
			banner.Pages = b.Pages
			modified = true
		}
		if !modified {
			continue
		}
		db.UpdateBanner(banner)
	}
}

func (s *Server) serveBanner(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	var err error
	data.Template = "manage_banner"
	data.Boards = db.AllBoards()
	data.Manage.Banner = &Banner{}

	deleteBannerID := PathInt(r, "/sriracha/banner/delete/")
	if deleteBannerID > 0 {
		if s.forbidden(w, data, "banner.delete") {
			return
		}
		b := db.BannerByID(deleteBannerID)
		if b == nil {
			data.ManageError("Invalid banner.")
			return
		}

		bannerPath := filepath.Join(s.config.Root, "banner", b.Name)
		os.Remove(bannerPath)

		db.DeleteBanner(b.ID)
		s.refreshBannerCache(db)
		s.rebuildAll(db)

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted banner #%d", b.ID), "")

		data.Redirect(w, r, "/sriracha/banner/")
		return
	}

	bannerID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/banner/"))
	if err == nil && bannerID > 0 {
		if s.forbidden(w, data, "banner.update") {
			return
		}
		data.Manage.Banner = db.BannerByID(bannerID)
		if data.Manage.Banner == nil {
			data.ManageError("Invalid banner.")
			return
		}

		if data.Manage.Banner != nil && r.Method == http.MethodPost {
			oldName := data.Manage.Banner.Name
			s.loadBannerForm(db, r, data.Manage.Banner)
			buf, err := s.loadBannerFormFile(db, r, data.Manage.Banner)
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			err = data.Manage.Banner.Validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			if data.Manage.Banner.Name != oldName {
				if buf == nil && filepath.Ext(data.Manage.Banner.Name) != filepath.Ext(oldName) {
					data.ManageError("File extension was modified in banner name: to change a banner's file extension, upload a new file")
					return
				}
				match := db.BannerByName(data.Manage.Banner.Name)
				if match != nil {
					data.ManageError("Banner with that name already exists")
					return
				}
				oldPath := filepath.Join(s.config.Root, "banner", oldName)
				newPath := filepath.Join(s.config.Root, "banner", data.Manage.Banner.Name)
				err = os.Rename(oldPath, newPath)
				if err != nil {
					log.Fatalf("failed to rename banner from %s to %s: %s", oldPath, newPath, err)
				}
			}

			if buf != nil {
				bannerPath := filepath.Join(s.config.Root, "banner", data.Manage.Banner.Name)
				err = os.WriteFile(bannerPath, buf, NewFilePermission)
				if err != nil {
					log.Fatalf("failed to write banner image at %s: %s", bannerPath, err)
				}
			}

			db.UpdateBanner(data.Manage.Banner)
			s.saveGlobalBannerSettings(db, data.Manage.Banner)
			s.refreshBannerCache(db)
			s.rebuildAll(db)

			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/banner/%d", data.Manage.Banner.ID), "")

			data.Redirect(w, r, "/sriracha/banner/")
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		if s.forbidden(w, data, "banner.add") {
			return
		}
		b := &Banner{}
		s.loadBannerForm(db, r, b)
		buf, err := s.loadBannerFormFile(db, r, b)
		if err != nil {
			data.ManageError(err.Error())
			return
		} else if buf == nil {
			data.ManageError("upload a file to add a banner")
			return
		}
		s.loadGlobalBannerSettings(db, b)

		err = b.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		match := db.BannerByName(b.Name)
		if match != nil {
			data.ManageError("Banner with that name already exists")
			return
		}

		bannerPath := filepath.Join(s.config.Root, "banner", b.Name)
		err = os.WriteFile(bannerPath, buf, NewFilePermission)
		if err != nil {
			log.Fatalf("failed to write banner image at %s: %s", bannerPath, err)
		}

		db.AddBanner(b)
		s.refreshBannerCache(db)
		s.rebuildAll(db)

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/banner/%d", b.ID), "")

		data.Redirect(w, r, "/sriracha/banner/")
		return
	}

	data.Manage.Banners = db.AllBanners()
}
