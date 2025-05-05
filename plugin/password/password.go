package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/tslocum/sriracha"
)

const (
	configMessage            = "message"
	configMessageDescription = "Error message shown when a post is denied."

	configPasswords            = "passwords"
	configPasswordsDescription = `Line 1 is the text visitors must enter in the password field.
Line 2 is optional, and may be set to a list of board IDs separated by commas.
When line 2 is set, the password only applies to the specified boards.
When line 2 is blank, the password applies to all boards.
Board IDs are available in the update board URL.
When no passwords exist for a board, normal access rules apply.`
)

type password struct {
	text   string
	boards []int
}

type Password struct {
	message   error
	passwords []*password
}

func (v *Password) About() string {
	return "Require specific passwords to post."
}

func (v *Password) Config() []sriracha.PluginConfig {
	return []sriracha.PluginConfig{
		{
			Type:        sriracha.TypeString,
			Name:        configMessage,
			Description: configMessageDescription,
			Default:     "Sorry, this board is locked. You must supply the correct password to submit a post.",
		},
		{
			Type:        sriracha.TypeString,
			Multiple:    true,
			Name:        configPasswords,
			Description: configPasswordsDescription,
		},
	}
}

func (v *Password) Update(db *sriracha.Database, key string) error {
	switch key {
	case configMessage:
		v.message = fmt.Errorf("%s", db.GetString(configMessage))
	case configPasswords:
		var passwords []*password
		for _, pass := range db.GetMultiString(configPasswords) {
			if strings.TrimSpace(pass) == "" {
				continue
			}
			split := strings.Split(pass, "\n")
			p := &password{
				text: split[0],
			}
			if len(split) > 1 {
				for _, vv := range strings.Split(split[1], ",") {
					boardID, err := strconv.Atoi(vv)
					if err == nil && boardID > 0 {
						p.boards = append(p.boards, boardID)
					}
				}
			}
			passwords = append(passwords, p)
		}
		v.passwords = passwords
	}
	return nil
}

func (v *Password) Post(db *sriracha.Database, post *sriracha.Post) error {
	var passwordRequired bool
	var passwordSubmitted bool
	for _, p := range v.passwords {
		if len(p.boards) != 0 && !slices.Contains(p.boards, post.Board.ID) {
			continue
		}
		passwordRequired = true
		if post.Password == p.text {
			passwordSubmitted = true
			post.Password = ""
			break
		}
	}
	if passwordRequired && !passwordSubmitted {
		return v.message
	}
	return nil
}

func init() {
	sriracha.RegisterPlugin(&Password{})
}

func main() {}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin           = &Password{}
	_ sriracha.PluginWithConfig = &Password{}
	_ sriracha.PluginWithUpdate = &Password{}
	_ sriracha.PluginWithPost   = &Password{}
)
