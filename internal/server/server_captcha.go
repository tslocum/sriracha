package server

import (
	"fmt"
	"image/color"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/steambap/captcha"
)

var captchaPalette = []color.Color{
	color.RGBA{0, 0, 0, 255},
}

func (s *Server) serveCAPTCHA(db serverDB, w http.ResponseWriter, r *http.Request) {
	const (
		width, height = 225, 40
		characters    = "ABCDHKMNSTUVWXYZ"
		refreshLimit  = 3
	)

	ipHash := s.hashIP(r)

	c := db.GetCAPTCHA(ipHash)
	if c != nil {
		queryValues := r.URL.Query()
		if len(queryValues["new"]) == 0 || c.Refresh >= refreshLimit {
			http.Redirect(w, r, fmt.Sprintf("/captcha/%s.png", c.Image), http.StatusFound)
			return
		}
	}

	challenge, err := captcha.New(width, height, func(options *captcha.Options) {
		options.CharPreset = characters
		options.TextLength = 5
		options.Noise = 1
		options.CurveNumber = 3
		options.Palette = captchaPalette
		options.BackgroundColor = color.Transparent
	})
	if err != nil {
		log.Fatal(err)
	}

	var oldImage string
	if c == nil {
		c = &CAPTCHA{
			IP:        ipHash,
			Timestamp: time.Now().Unix(),
		}
	} else {
		oldImage = c.Image
		c.Refresh++
	}

	c.Image = db.NewCAPTCHAImage()
	c.Text = strings.ToLower(challenge.Text)

	f, err := os.OpenFile(filepath.Join(s.config.Root, "captcha", c.Image+".png"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}
	challenge.WriteImage(f)
	f.Close()

	if oldImage != "" {
		os.Remove(filepath.Join(s.config.Root, "captcha", oldImage+".png"))
	}

	if oldImage == "" {
		db.AddCAPTCHA(c)
	} else {
		db.UpdateCAPTCHA(c)
	}

	s.captchaCacheLock.Lock()
	s.captchaCache[c.IP] = c.Image
	s.captchaCacheLock.Unlock()

	http.Redirect(w, r, fmt.Sprintf("/captcha/%s.png", c.Image), http.StatusFound)
}
