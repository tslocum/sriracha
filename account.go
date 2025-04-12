package sriracha

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AccountRole int

const (
	RoleSuperAdmin AccountRole = 1
	RoleAdmin      AccountRole = 2
	RoleMod        AccountRole = 3
	RoleDisabled   AccountRole = 99
)

type Account struct {
	ID         int
	Username   string
	Role       AccountRole
	LastActive int64
	Session    string
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

func (a *Account) loadForm(r *http.Request) {
	a.Username = strings.TrimSpace(r.FormValue("username"))
	roleString := r.FormValue("role")
	switch roleString {
	case "1":
		a.Role = RoleSuperAdmin
	case "2":
		a.Role = RoleAdmin
	case "3":
		a.Role = RoleMod
	default:
		a.Role = RoleDisabled
	}
}

func (a *Account) LastActiveDate() string {
	if a.LastActive == 0 {
		return "Never"
	}
	return time.Unix(a.LastActive, 0).Format("2006-01-02 15:04:05 MST")
}
