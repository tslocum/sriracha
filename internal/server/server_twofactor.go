package server

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	totpPeriod         = 30
	totpSecretSize     = 32
	totpDigits         = 6
	totpSkew           = 1
	totpImageSize      = 250
	totpKeySize        = 48
	totpAlgorithm      = otp.AlgorithmSHA512
	totpSessionTimeout = 600 // 10 minutes.
	totpMaxDevices     = 5
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

func (s *Server) twoFactorSession(accountID int, key []byte) *twoFactorSession {
	var existing *twoFactorSession
	now := time.Now().Unix()
	s.twoFactorSessions = slices.DeleteFunc(s.twoFactorSessions, func(twoFactor *twoFactorSession) bool {
		return now-twoFactor.timestamp > totpSessionTimeout
	})
	if len(key) > 0 {
		for _, session := range s.twoFactorSessions {
			if bytes.Equal(session.key, key) {
				existing = session
				break
			}
		}
	}
	s.twoFactorSessions = slices.DeleteFunc(s.twoFactorSessions, func(twoFactor *twoFactorSession) bool {
		return (twoFactor != existing && twoFactor.account == accountID) || (twoFactor == existing && twoFactor.account != accountID)
	})
	if existing != nil {
		existing.timestamp = now
		return existing
	}
	session := &twoFactorSession{
		key:       s.twoFactorKey(),
		account:   accountID,
		timestamp: now,
	}
	s.twoFactorSessions = append(s.twoFactorSessions, session)
	return session
}

func (s *Server) addTwoFactorNotice(data *templateData, db serverDB) {
	if !s.config.Require2FA || len(db.TwoFactorsByAccount(data.Account.ID)) > 0 || data.Info != "" {
		return
	}
	data.Info = data.Get("Two-factor authentication is required. You may only access your preferences until a %s device is added.", "2FA")
}

func (s *Server) serveTwoFactor(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_twofactor"
	data.Manage.TwoFactor = &TwoFactor{}
	key := []byte(FormString(r, "key"))
	if len(key) == 0 {
		cookies := r.CookiesNamed("sriracha_totp")
		if len(cookies) > 0 {
			key = []byte(cookies[0].Value)
		}
	}
	session := s.twoFactorSession(data.Account.ID, key)
	data.Extra3 = string(session.key)
	password := FormString(r, "password")
	if password != "" {
		match := db.CheckAccountPassword(data.Account.Username, password)
		if match == nil {
			data.ManageError("Incorrect password")
			return
		}
		session.loggedIn = true
	}
	if !session.loggedIn {
		data.ManageError("Incorrect password")
		return
	}
	allDevices := db.TwoFactorsByAccount(data.Account.ID)
	if !session.validated && len(allDevices) > 0 {
		passcode := FormString(r, "passcode")
		if passcode == "" {
			data.Extra2 = "passcode"
			return
		}
		now := time.Now()
		for _, device := range allDevices {
			ok, err := totp.ValidateCustom(passcode, device.Secret, now, twoFactorValidateOptions)
			if err == nil && ok {
				session.validated = true
				break
			}
		}
		if !session.validated {
			data.ManageError("Incorrect passcode")
			return
		}
		session.validated = true
	}
	if !session.loggedIn {
		data.Redirect(w, r, "/sriracha/preference")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/sriracha/preference/2fa/add") {
		if len(db.TwoFactorsByAccount(data.Account.ID)) >= totpMaxDevices {
			data.ManageError(data.Get("Sorry, only %d devices may be added. Remove a device before adding another.", totpMaxDevices))
			return
		} else if data.Manage.TwoFactor.Secret == "" && session.secret != "" {
			data.Manage.TwoFactor.Secret = session.secret
		}
		passcode := FormString(r, "passcode")
		if passcode != "" && session.secret != "" {
			ok, err := totp.ValidateCustom(passcode, session.secret, time.Now(), twoFactorValidateOptions)
			if err != nil || !ok {
				data.ManageError("Incorrect passcode")
				return
			}
			now := time.Now().Unix()
			t := &TwoFactor{
				Account:    data.Account.ID,
				Timestamp:  now,
				LastActive: now,
				Secret:     session.secret,
			}
			db.AddTwoFactor(t)
			session.validated = true
			session.secret = ""
			data.Template = "manage_info"
			data.Info = data.Get("Added %s device.", "2FA")
			return
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
		return
	}
	session.secret = ""

	// Rename device.
	deviceID := PathInt(r, "/sriracha/preference/2fa/")
	if deviceID > 0 {
		device := db.TwoFactorByID(deviceID)
		if device == nil || device.Account != data.Account.ID {
			data.ManageError("Invalid or removed device.")
			return
		}
		data.Manage.TwoFactor = device
		if FormString(r, "rename") != "" {
			data.Manage.TwoFactor.Name = FormString(r, "name")
			db.UpdateTwoFactor(data.Manage.TwoFactor)
			data.Template = "manage_info"
			data.Info = "Renamed device"
			return
		}
	}

	// Delete device.
	deviceID = FormInt(r, "delete")
	if deviceID > 0 {
		device := db.TwoFactorByID(deviceID)
		if device == nil || device.Account != data.Account.ID {
			data.ManageError("Invalid or removed device.")
			return
		}
		db.DeleteTwoFactor(device.ID)
		s.addTwoFactorNotice(data, db)
	}

	data.Manage.TwoFactors = db.TwoFactorsByAccount(data.Account.ID)
}
