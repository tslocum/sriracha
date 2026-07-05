// Package model provides Sriracha data types.
package model

import (
	"html/template"

	"codeberg.org/tslocum/gotext"
)

func Locale(identifier string) string {
	if identifier == "" {
		return "sriracha-en"
	}
	return "sriracha-" + identifier
}

func G(board *Board, account *Account, str string) string {
	var locale string
	if account != nil && account.Locale != "" {
		locale = Locale(account.Locale)
	} else if board != nil && board.Locale != "" {
		locale = Locale(board.Locale)
	} else {
		return gotext.G(str)
	}
	return gotext.GD(locale, str)
}

func Get(board *Board, account *Account, str string, vars ...interface{}) string {
	var locale string
	if account != nil && account.Locale != "" {
		locale = Locale(account.Locale)
	} else if board != nil && board.Locale != "" {
		locale = Locale(board.Locale)
	} else {
		return gotext.Get(str, vars...)
	}
	return gotext.GetD(locale, str, vars...)
}

func GetHTML(board *Board, account *Account, str string, vars ...interface{}) template.HTML {
	return template.HTML(Get(board, account, str, vars...))
}

func GetN(board *Board, account *Account, singular string, plural string, v int) string {
	var locale string
	if account != nil && account.Locale != "" {
		locale = Locale(account.Locale)
	} else if board != nil && board.Locale != "" {
		locale = Locale(board.Locale)
	} else {
		return gotext.GetN(singular, plural, v, v)
	}
	return gotext.GetND(locale, singular, plural, v, v)
}
