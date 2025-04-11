package sriracha

import "embed"

//go:embed template
var templatesFS embed.FS

type templateData struct {
	Account *Account
}

var guestData = &templateData{}
