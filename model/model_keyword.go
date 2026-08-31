package model

import (
	"fmt"
	"slices"
	"strings"

	. "codeberg.org/tslocum/sriracha/util"
	"github.com/dlclark/regexp2/v2/compat"
)

type Keyword struct {
	ID     int
	Text   string
	Action string
	Boards []*Board `diff:"-"`
}

func (k *Keyword) Validate() error {
	switch {
	case strings.TrimSpace(k.Text) == "":
		return fmt.Errorf("text must be set")
	case !slices.Contains(AllActions, k.Action):
		return fmt.Errorf("action must be set")
	}
	_, err := compat.Compile(k.Text)
	if err != nil {
		return fmt.Errorf("keyword `%s` is invalid: %s", k.Text, err)
	}
	return nil
}

func (k *Keyword) HasBoard(id int) bool {
	for _, b := range k.Boards {
		if b.ID == id {
			return true
		}
	}
	return false
}

func (k *Keyword) ActionLabel(account *Account) string {
	return G(nil, account, FormatAction(k.Action))
}
