package sriracha

type CAPTCHA struct {
	IP        string
	Timestamp int64
	Refresh   int
	Image     string
	Text      string
}
