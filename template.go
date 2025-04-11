package sriracha

import "embed"

//go:embed template
var templateFS embed.FS

type templateData struct {
	Account *Account
	Info    string
	Error   string
}

var guestData = &templateData{}
