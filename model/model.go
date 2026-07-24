// Package model provides Sriracha data types.
package model

import (
	"hash/crc32"
	"html/template"

	"codeberg.org/tslocum/gotext"
)

var (
	crcHash = crc32.NewIEEE()
	crcBuf  = make([]byte, 8)
	crcSum  []byte
	CRCSalt []byte
)

// AllActions is a list of all automated moderation actions.
var AllActions = []string{"hide", "report", "delete", "ban1h", "ban1d", "ban2d", "ban1w", "ban2w", "ban1m", "ban0"}

// Domain returns the gettext domain name corresponding to the specified locale.
func Domain(locale string) string {
	if locale == "" {
		return "sriracha-en"
	}
	return "sriracha-" + locale
}

func G(board *Board, account *Account, str string) string {
	var domain string
	if account != nil && account.Locale != "" {
		domain = Domain(account.Locale)
	} else if board != nil && board.Locale != "" {
		domain = Domain(board.Locale)
	} else {
		return gotext.G(str)
	}
	return gotext.GD(domain, str)
}

func Get(board *Board, account *Account, str string, vars ...interface{}) string {
	var domain string
	if account != nil && account.Locale != "" {
		domain = Domain(account.Locale)
	} else if board != nil && board.Locale != "" {
		domain = Domain(board.Locale)
	} else {
		return gotext.Get(str, vars...)
	}
	return gotext.GetD(domain, str, vars...)
}

func GetHTML(board *Board, account *Account, str string, vars ...interface{}) template.HTML {
	return template.HTML(Get(board, account, str, vars...))
}

func GetN(board *Board, account *Account, singular string, plural string, v int) string {
	var domain string
	if account != nil && account.Locale != "" {
		domain = Domain(account.Locale)
	} else if board != nil && board.Locale != "" {
		domain = Domain(board.Locale)
	} else {
		return gotext.GetN(singular, plural, v, v)
	}
	return gotext.GetND(domain, singular, plural, v, v)
}
