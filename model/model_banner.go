package model

import (
	"fmt"
	"strings"

	. "codeberg.org/tslocum/sriracha/util"
)

type Banner struct {
	ID        int
	Name      string
	Width     int
	Height    int
	Overboard bool
	News      bool
	Pages     bool
	Boards    []*Board `diff:"-"`
}

func (b *Banner) Validate() error {
	b.Name = strings.TrimSpace(b.Name)
	switch {
	case strings.TrimSpace(b.Name) == "" || !FilePathPattern.MatchString(b.Name) || strings.HasPrefix(b.Name, "."):
		return fmt.Errorf("invalid banner name: %s", b.Name)
	case b.Width <= 0:
		return fmt.Errorf("invalid width")
	case b.Height <= 0:
		return fmt.Errorf("invalid height")
	}
	return nil
}

func (b *Banner) HasBoard(id int) bool {
	for _, board := range b.Boards {
		if board.ID == id {
			return true
		}
	}
	return false
}
