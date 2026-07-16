package server

import (
	"crypto/rand"
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

func (s *Server) twoFactorOptions(a *Account, t *TwoFactor) totp.GenerateOpts {
	opts := totp.GenerateOpts{
		Issuer:      s.opt.SiteName,
		AccountName: a.Username,
		Period:      totpPeriod,
		SecretSize:  totpSecretSize,
		Digits:      totpDigits,
		Algorithm:   otp.AlgorithmSHA512,
		Rand:        rand.Reader,
	}
	if t.Secret != "" {
		opts.Secret = []byte(t.Secret)
	}
	return opts
}

func (s *Server) serveTwoFactor(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_twofactor"
	data.Manage.TwoFactor = &TwoFactor{}
	if strings.HasPrefix(r.URL.Path, "/sriracha/preference/2fa/add") {
		data.Manage.TwoFactor.Timestamp = time.Now().Unix()
		data.Manage.TwoFactor.Secret = "test"
	}
}
