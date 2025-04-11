package sriracha

import (
	"fmt"
	"strings"
)

type BoardType int

const (
	TypeImageboard BoardType = 0
	TypeForum      BoardType = 1
)

type Board struct {
	ID          int
	Dir         string
	Name        string
	Description string
	Type        BoardType
}

func (b *Board) validate() error {
	switch {
	case strings.TrimSpace(b.Dir) == "":
		return fmt.Errorf("dir must be set")
	case strings.TrimSpace(b.Name) == "":
		return fmt.Errorf("name must be set")
	case !alphaNumericAndSymbols.MatchString(b.Dir):
		return fmt.Errorf("dir must only consist of letters, numbers, hyphens and underscores")
	}
	return nil
}
