package server

import (
	"crypto/rand"
	"encoding/base32"
	"log"
	"net/http"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	totpPeriod     = 30
	totpSecretSize = 32
	totpDigits     = 6
	totpImageSize  = 250
)

var b32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

func (s *Server) twoFactorOptions(a *Account, t *TwoFactor) totp.GenerateOpts {
	opts := totp.GenerateOpts{
		Issuer:      s.opt.SiteName,
		AccountName: a.Username,
		Period:      totpPeriod,
		Digits:      totpDigits,
		Algorithm:   otp.AlgorithmSHA512,
		Rand:        rand.Reader,
	}
	if t.Secret != "" {
		var err error
		opts.Secret, err = b32NoPadding.DecodeString(t.Secret)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		opts.SecretSize = totpSecretSize
	}
	return opts
}

func (s *Server) serveTwoFactor(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_twofactor"
	data.Manage.TwoFactor = &TwoFactor{}
	if strings.HasPrefix(r.URL.Path, "/sriracha/preference/2fa/add") {
		data.Manage.TwoFactor.Timestamp = time.Now().Unix()
		key, err := totp.Generate(s.twoFactorOptions(data.Account, data.Manage.TwoFactor))
		if err != nil {
			log.Fatal(err)
		}
		data.Manage.TwoFactor.Secret = key.Secret()
	}
}
