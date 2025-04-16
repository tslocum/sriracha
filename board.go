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

type BoardApproval int

const (
	ApprovalNone BoardApproval = 0
	ApprovalFile BoardApproval = 1
	ApprovalAll  BoardApproval = 2
)

type Board struct {
	ID          int
	Dir         string
	Name        string
	Description string
	Type        BoardType
	Approval    BoardApproval
	MaxSize     int64
	ThumbWidth  int
	ThumbHeight int
	Unique      int
}

const (
	defaultBoardMaxSize     = 2097152
	defaultBoardThumbWidth  = 250
	defaultBoardThumbHeight = 250
)

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
	b.Dir = formString(r, "dir")
	b.Name = formString(r, "name")
	b.Description = formString(r, "description")
	b.Type = formRange(r, "type", TypeImageboard, TypeForum)
	b.Approval = formRange(r, "approval", ApprovalNone, ApprovalAll)
	b.MaxSize = formInt64(r, "maxsize")
	b.ThumbWidth = formInt(r, "thumbwidth")
	b.ThumbHeight = formInt(r, "thumbheight")
}

func (b *Board) MaxSizeLabel() string {
	const base = 1024
	var sizes = []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	unitsLimit := len(sizes)

	var i int
	size := b.MaxSize
	for i := 0; size >= base && i < unitsLimit; i++ {
		size = size / base
		i++
	}
	return fmt.Sprintf("%d %s", size, sizes[i])
}
