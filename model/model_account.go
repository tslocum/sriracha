package model

import (
	"fmt"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/util"
)

type AccountRole int

// Account roles.
const (
	RoleSuperAdmin AccountRole = 1
	RoleAdmin      AccountRole = 2
	RoleMod        AccountRole = 3
	RoleDisabled   AccountRole = 99
)

func FormatRole(role AccountRole) string {
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
	Style      string
	Locale     string
}

func (a *Account) Validate() error {
	switch {
	case strings.TrimSpace(a.Username) == "":
		return fmt.Errorf("username must be set")
	case !AlphanumericAndSymbols.MatchString(a.Username):
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

func (a *Account) Anonymize() {
	aa := &Account{
		ID: a.ID,
	}
	*a = *aa
}
