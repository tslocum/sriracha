package sriracha

import (
	"fmt"
	"net/http"
	"strings"
)

type BoardType int

// Board types.
const (
	TypeImageboard BoardType = 0
	TypeForum      BoardType = 1
)

func formatBoardType(t BoardType) string {
	switch t {
	case TypeImageboard:
		return "Imageboard"
	case TypeForum:
		return "Forum"
	default:
		return "Unknown"
	}
}

type BoardLock int

// Board lock types.
const (
	LockNone   BoardLock = 0
	LockReply  BoardLock = 1
	LockThread BoardLock = 2
	LockAll    BoardLock = 3
)

func formatBoardLock(l BoardLock) string {
	switch l {
	case LockNone:
		return "None"
	case LockReply:
		return "Reply"
	case LockThread:
		return "Thread"
	case LockAll:
		return "All"
	default:
		return "Unknown"
	}
}

type BoardApproval int

const (
	ApprovalNone BoardApproval = 0
	ApprovalFile BoardApproval = 1
	ApprovalAll  BoardApproval = 2
)

func formatBoardApproval(a BoardApproval) string {
	switch a {
	case ApprovalNone:
		return "None"
	case ApprovalFile:
		return "File"
	case ApprovalAll:
		return "All"
	default:
		return "Unknown"
	}
}

type Board struct {
	ID          int
	Dir         string
	Name        string
	Description string
	Type        BoardType
	Lock        BoardLock
	Approval    BoardApproval
	Reports     bool
	Locale      string
	Delay       int
	Threads     int
	Replies     int
	MaxName     int
	MaxEmail    int
	MaxSubject  int
	MaxMessage  int
	MaxThreads  int
	MaxReplies  int
	DefaultName string
	WordBreak   int
	Truncate    int
	MaxSize     int64
	ThumbWidth  int
	ThumbHeight int

	// Calculated fields.
	Unique int
}

const (
	defaultBoardThreads     = 10
	defaultBoardReplies     = 3
	defaultBoardMaxName     = 75
	defaultBoardMaxEmail    = 255
	defaultBoardMaxSubject  = 75
	defaultBoardMaxMessage  = 8000
	defaultBoardWordBreak   = 80
	defaultBoardDefaultName = "Anonymous"
	defaultBoardTruncate    = 15
	defaultBoardMaxSize     = 2097152
	defaultBoardThumbWidth  = 250
	defaultBoardThumbHeight = 250
)

func (b *Board) loadForm(r *http.Request) {
	b.Dir = formString(r, "dir")
	b.Name = formString(r, "name")
	b.Description = formString(r, "description")
	b.Type = formRange(r, "type", TypeImageboard, TypeForum)
	b.Lock = formRange(r, "lock", LockNone, LockAll)
	b.Approval = formRange(r, "approval", ApprovalNone, ApprovalAll)
	b.Reports = formBool(r, "reports")
	b.Locale = formString(r, "locale")
	b.Delay = formInt(r, "delay")
	b.Threads = formInt(r, "threads")
	b.Replies = formInt(r, "replies")
	b.MaxName = formInt(r, "maxname")
	b.MaxEmail = formInt(r, "maxemail")
	b.MaxSubject = formInt(r, "maxsubject")
	b.MaxMessage = formInt(r, "maxmessage")
	b.MaxThreads = formInt(r, "maxthreads")
	b.MaxReplies = formInt(r, "maxreplies")
	b.DefaultName = formString(r, "defaultname")
	b.WordBreak = formInt(r, "wordbreak")
	b.Truncate = formInt(r, "truncate")
	b.MaxSize = formInt64(r, "maxsize")
	b.ThumbWidth = formInt(r, "thumbwidth")
	b.ThumbHeight = formInt(r, "thumbheight")
}

func (b *Board) validate() error {
	switch {
	case b.Dir != "" && !alphaNumericAndSymbols.MatchString(b.Dir):
		return fmt.Errorf("dir must only consist of letters, numbers, hyphens and underscores")
	case strings.TrimSpace(b.Name) == "":
		return fmt.Errorf("name must be set")
	}
	reservedDirs := []string{"captcha", "css", "js", "sriracha", "sriracha_all"}
	for _, reserved := range reservedDirs {
		if strings.EqualFold(b.Dir, reserved) {
			return fmt.Errorf("%s is a reserved name", reserved)
		}
	}
	return nil
}

func (b *Board) Path() string {
	if b.Dir == "" {
		return "/"
	}
	return "/" + b.Dir + "/"
}

func (b *Board) MaxSizeLabel() string {
	return formatFileSize(b.MaxSize)
}
