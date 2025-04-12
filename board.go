package sriracha

import (
	"fmt"
	"net/http"
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
	case strings.EqualFold(b.Dir, "imgboard"):
		return fmt.Errorf("imgboard is a reserved name")
	case strings.EqualFold(b.Dir, "sriracha_all"):
		return fmt.Errorf("sriracha_all is a reserved name")
	}
	return nil
}

func (b *Board) loadForm(r *http.Request) {
	b.Dir = strings.TrimSpace(r.FormValue("dir"))
	b.Name = strings.TrimSpace(r.FormValue("name"))
	b.Description = strings.TrimSpace(r.FormValue("description"))
	typeString := r.FormValue("type")
	if typeString == "1" {
		b.Type = TypeForum
	} else {
		b.Type = TypeImageboard
	}
}
