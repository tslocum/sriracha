package sriracha

import (
	"embed"
	"html/template"
)

//go:embed template
var templateFS embed.FS

type manageData struct {
	Account  *Account
	Accounts []*Account
	Ban      *Ban
	Bans     []*Ban
	Board    *Board
	Boards   []*Board
	Keyword  *Keyword
	Keywords []*Keyword
	Log      *Log
	Logs     []*Log
	Plugin   *pluginInfo
	Plugins  []*pluginInfo
}

type templateData struct {
	Account  *Account
	Info     string
	Message  template.HTML
	Board    *Board
	Boards   []*Board
	Manage   *manageData
	Template string
}

func (data *templateData) Error(message string) {
	data.Template = "manage_error"
	data.Info = message
}

var guestData = &templateData{
	Manage: &manageData{},
}
