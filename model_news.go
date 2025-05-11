package sriracha

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type News struct {
	ID        int
	Account   *Account
	Timestamp int64
	Modified  int64
	Share     bool
	Name      string
	Subject   string
	Message   string
}

func (n *News) validate() error {
	switch {
	case strings.TrimSpace(n.Message) == "":
		return fmt.Errorf("a message is required")
	case n.Timestamp < 0:
		return fmt.Errorf("invalid news timestamp")
	default:
		return nil
	}
}

func (n *News) loadForm(db *Database, r *http.Request, a *Account) {
	n.Timestamp = formInt64(r, "timestamp")
	if n.Account != nil && n.Account.ID == a.ID {
		n.Share = formBool(r, "share")
	}
	n.Name = formString(r, "name")
	n.Subject = formString(r, "subject")
	n.Message = formString(r, "message")
}

func (n *News) MayUpdate(a *Account) bool {
	if a == nil {
		return false
	}
	return n.Share || (n.Account != nil && n.Account.ID == a.ID) || (n.Account == nil && (a.Role == RoleSuperAdmin || a.Role == RoleAdmin))
}

func (n *News) MayDelete(a *Account) bool {
	if a == nil {
		return false
	}
	return a.Role == RoleSuperAdmin || a.Role == RoleAdmin || n.MayUpdate(a)
}

func (n *News) DateLabel() string {
	switch {
	case n.Timestamp == 0:
		return "Draft"
	case n.Timestamp > time.Now().Unix():
		return "Hidden until " + FormatTimestamp(n.Timestamp)
	default:
		return FormatTimestamp(n.Timestamp)
	}
}
