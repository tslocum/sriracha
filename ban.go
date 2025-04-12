package sriracha

import (
	"fmt"
	"net/http"
	"strconv"
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

func (b *Ban) validate() error {
	switch {
	case strings.TrimSpace(b.IP) == "":
		return fmt.Errorf("IP must be set")
	case b.Expire < 0:
		return fmt.Errorf("expiraton must be greater than or equal to zero")
	}
	return nil
}

func (b *Ban) loadForm(r *http.Request) {
	expire, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("expire")), 10, 64)
	if err == nil && expire >= 0 {
		b.Expire = expire
	}
	b.Reason = strings.TrimSpace(r.FormValue("reason"))
}

func (b *Ban) ExpireDate() string {
	if b.Expire == 0 {
		return "Never"
	}
	return time.Unix(b.Expire, 0).Format("2006-01-02 15:04:05 MST")
}
