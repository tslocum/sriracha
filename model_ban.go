package sriracha

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Ban struct {
	ID        int
	IP        string
	Timestamp int64
	Expire    int64
	Reason    string
}

func (b *Ban) loadForm(r *http.Request) {
	b.Expire = formInt64(r, "expire")
	b.Reason = formString(r, "reason")
}

func (b *Ban) validate() error {
	switch {
	case strings.TrimSpace(b.IP) == "":
		return fmt.Errorf("IP must be set")
	case b.Expire < 0:
		return fmt.Errorf("expiraton must be greater than or equal to zero")
	}
	return nil
}

func (b *Ban) ExpireDate() string {
	if b.Expire == 0 {
		return "Never"
	}
	return time.Unix(b.Expire, 0).Format("2006-01-02 15:04:05 MST")
}

func (b *Ban) Info() string {
	var info string
	if b.Expire == 0 {
		info += "This ban is permanent."
	} else {
		info += fmt.Sprintf("This ban will expire at %s.", FormatTimestamp(b.Expire))
	}
	if b.Reason != "" {
		info += " Reason: " + b.Reason
	}
	return info
}
