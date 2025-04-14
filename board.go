package sriracha

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type BoardType int

const (
	TypeImageboard BoardType = 0
	TypeForum      BoardType = 1
)

type BoardApprovalType int

const (
	ApprovalNone BoardApprovalType = 0
	ApprovalFile BoardApprovalType = 1
	ApprovalAll  BoardApprovalType = 2
)

type Board struct {
	ID          int
	Dir         string
	Name        string
	Description string
	Type        BoardType
	Approval    BoardApprovalType
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
	b.Dir = strings.TrimSpace(r.FormValue("dir"))
	b.Name = strings.TrimSpace(r.FormValue("name"))
	b.Description = strings.TrimSpace(r.FormValue("description"))
	typeString := r.FormValue("type")
	if typeString == "1" {
		b.Type = TypeForum
	} else {
		b.Type = TypeImageboard
	}
	approvalString := r.FormValue("approval")
	if approvalString == "1" {
		b.Approval = ApprovalFile
	} else if approvalString == "2" {
		b.Approval = ApprovalAll
	} else {
		b.Approval = ApprovalNone
	}
	maxSizeString := strings.TrimSpace(r.FormValue("maxsize"))
	if maxSizeString != "" {
		v, err := strconv.ParseInt(maxSizeString, 10, 64)
		if err == nil && v >= 0 {
			b.MaxSize = v
		}
	}
	thumbWidthString := strings.TrimSpace(r.FormValue("thumbwidth"))
	if thumbWidthString != "" {
		v, err := strconv.Atoi(thumbWidthString)
		if err == nil && v >= 0 {
			b.ThumbWidth = v
		}
	}
	thumbHeightString := strings.TrimSpace(r.FormValue("thumbheight"))
	if thumbHeightString != "" {
		v, err := strconv.Atoi(thumbHeightString)
		if err == nil && v >= 0 {
			b.ThumbHeight = v
		}
	}
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
