package sriracha

import "embed"

//go:embed template
var templateFS embed.FS

type manageData struct {
	Board  *Board
	Boards []*Board
}

type templateData struct {
	Account *Account
	Info    string
	Error   string
	Board   *Board
	Manage  *manageData
}

var guestData = &templateData{
	Manage: &manageData{},
}
