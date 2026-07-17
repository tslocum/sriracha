package server

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"log"
	"net/http"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	totpPeriod          = 30
	totpSecretSize      = 32
	totpDigits          = 6
	totpSkew            = 1
	totpImageSize       = 250
	totpKeySize         = 48
	totpAlgorithm       = otp.AlgorithmSHA512
	totpSessionDuration = 600 // 10 minutes.
	totpMaxDevices      = 5
)

var b32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

var twoFactorValidateOptions = totp.ValidateOpts{
	Period:    totpPeriod,
	Skew:      totpSkew,
	Digits:    totpDigits,
	Algorithm: totpAlgorithm,
}

func (s *Server) twoFactorOptions(a *Account, t *TwoFactor) totp.GenerateOpts {
	opts := totp.GenerateOpts{
		Issuer:      s.opt.SiteName,
		AccountName: a.Username,
		Period:      totpPeriod,
		Digits:      totpDigits,
		Algorithm:   totpAlgorithm,
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

func (s *Server) twoFactorKey() []byte {
TWOFACTORKEY:
	for {
		key := []byte(randomString(totpKeySize))
		for _, session := range s.twoFactorSessions {
			if bytes.Equal(key, session.key) {
				continue TWOFACTORKEY
			}
		}
		return key
	}
}

func (s *Server) twoFactorSession(a *Account, key []byte) *twoFactorSession {
	if a.ID == 0 {
		return &twoFactorSession{}
	}
	var existing *twoFactorSession
	var index int
	for i, session := range s.twoFactorSessions {
		if session.account == a.ID {
			existing = s.twoFactorSessions[i]
			index = i
			break
		}
	}
	if existing != nil {
		if bytes.Equal(existing.key, key) && time.Now().Unix()-existing.timestamp <= totpSessionDuration {
			return existing
		}
		s.twoFactorSessions = append(s.twoFactorSessions[:index], s.twoFactorSessions[index+1:]...)
	}
	session := &twoFactorSession{
		key:       s.twoFactorKey(),
		account:   a.ID,
		timestamp: time.Now().Unix(),
	}
	s.twoFactorSessions = append(s.twoFactorSessions, session)
	return session
}

func (s *Server) serveTwoFactor(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_twofactor"
	data.Manage.TwoFactor = &TwoFactor{}
	key := []byte(FormString(r, "key"))
	session := s.twoFactorSession(data.Account, key)
	password := FormString(r, "password")
	if password != "" {
		match := db.CheckAccountPassword(data.Account.Username, password)
		if match == nil {
			data.ManageError("Incorrect password")
			return
		}
		session.timestamp = time.Now().Unix()
		session.complete = true
	}
	if !session.complete {
		data.Redirect(w, r, "/sriracha/preference")
		return
	} else if data.Manage.TwoFactor.Secret == "" && session.secret != "" {
		data.Manage.TwoFactor.Secret = session.secret
	}
	data.Extra3 = string(session.key)
	if strings.HasPrefix(r.URL.Path, "/sriracha/preference/2fa/add") {
		if len(db.TwoFactorsByAccount(data.Account.ID)) >= totpMaxDevices {
			data.ManageError(data.Get("Sorry, only %d devices may be added. Remove a device before adding another.", totpMaxDevices))
			return
		}
		passcode := FormString(r, "passcode")
		if passcode != "" && session.secret != "" {
			ok, err := totp.ValidateCustom(passcode, session.secret, time.Now(), twoFactorValidateOptions)
			if err != nil || !ok {
				data.ManageError("Incorrect passcode")
				return
			}
			// TODO Valid, add to DB and redirect
		}
		data.Manage.TwoFactor.Timestamp = time.Now().Unix()
		options := s.twoFactorOptions(data.Account, data.Manage.TwoFactor)
		key, err := totp.Generate(options)
		if err != nil {
			log.Fatal(err)
		}
		if data.Manage.TwoFactor.Secret == "" {
			data.Manage.TwoFactor.Secret = key.Secret()
			session.secret = data.Manage.TwoFactor.Secret
		}
	}
}
