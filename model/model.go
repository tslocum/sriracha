package model

import "github.com/leonelquinteros/gotext"

func Get(board *Board, account *Account, str string, vars ...interface{}) string {
	var locale string
	if account != nil && account.Locale != "" {
		locale = "sriracha-" + account.Locale
	} else if board != nil && board.Locale != "" {
		locale = "sriracha-" + board.Locale
	} else {
		locale = "sriracha"
	}
	return gotext.GetD(locale, str, vars...)
}
