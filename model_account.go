package sriracha

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AccountRole int

// Account roles.
const (
	RoleSuperAdmin AccountRole = 1
	RoleAdmin      AccountRole = 2
	RoleMod        AccountRole = 3
	RoleDisabled   AccountRole = 99
)

func formatRole(role AccountRole) string {
	switch role {
	case RoleSuperAdmin:
		return "Super-administrator"
	case RoleAdmin:
		return "Administrator"
	case RoleMod:
		return "Moderator"
	case RoleDisabled:
		return "Disabled"
	default:
		return "Unknown"
	}
}

type Account struct {
	ID         int
	Username   string
	Password   string
	Role       AccountRole
	LastActive int64
	Session    string
	Style      string
}

func (a *Account) loadForm(r *http.Request) {
	a.Username = formString(r, "username")
	a.Role = formRange(r, "role", RoleSuperAdmin, RoleDisabled)
}

func (a *Account) validate() error {
	switch {
	case strings.TrimSpace(a.Username) == "":
		return fmt.Errorf("username must be set")
	case !alphaNumericAndSymbols.MatchString(a.Username):
		return fmt.Errorf("username must only consist of letters, numbers, hyphens and underscores")
	}
	return nil
}

func (a *Account) LastActiveDate() string {
	if a.LastActive == 0 {
		return "Never"
	}
	return time.Unix(a.LastActive, 0).Format("2006-01-02 15:04:05 MST")
}
