package sriracha

import "embed"

//go:embed template
var templateFS embed.FS

type templateData struct {
	Account *Account
}

var guestData = &templateData{}
