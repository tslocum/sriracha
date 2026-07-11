package model

import (
	"fmt"
	"strings"

	. "codeberg.org/tslocum/sriracha/util"
)

type BoardType int

// Board types.
const (
	TypeImageboard BoardType = 0
	TypeForum      BoardType = 1
)

func FormatBoardType(t BoardType) string {
	switch t {
	case TypeImageboard:
		return "Imageboard"
	case TypeForum:
		return "Forum"
	default:
		return "Unknown"
	}
}

type BoardHide int

const (
	HideNowhere    BoardHide = 0
	HideIndex      BoardHide = 1
	HideOverboard  BoardHide = 2
	HideEverywhere BoardHide = 3
)

func FormatBoardHide(a BoardHide) string {
	switch a {
	case HideNowhere:
		return "Show everywhere"
	case HideIndex:
		return "Hide from index"
	case HideOverboard:
		return "Hide from overboard"
	case HideEverywhere:
		return "Hide everywhere"
	default:
		return "Unknown"
	}
}

type BoardLock int

// Board lock types.
const (
	LockNone   BoardLock = 0
	LockThread BoardLock = 1
	LockPost   BoardLock = 2
	LockStaff  BoardLock = 3
)

func FormatBoardLock(l BoardLock) string {
	switch l {
	case LockNone:
		return "Allow all"
	case LockThread:
		return "No visitor threads"
	case LockPost:
		return "No visitor posts"
	case LockStaff:
		return "No posts"
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

func FormatBoardApproval(a BoardApproval) string {
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

type BoardIdentifiers int

const (
	IdentifiersDisable BoardIdentifiers = 0
	IdentifiersBoard   BoardIdentifiers = 1
	IdentifiersGlobal  BoardIdentifiers = 2
)

func FormatBoardIdentifiers(i BoardIdentifiers) string {
	switch i {
	case IdentifiersDisable:
		return "Disable"
	case IdentifiersBoard:
		return "Board"
	case IdentifiersGlobal:
		return "Global"
	default:
		return "Unknown"
	}
}

type BoardRequire int

const (
	RequireNever   BoardRequire = 0
	RequireThreads BoardRequire = 1
	RequireReplies BoardRequire = 2
	RequireAll     BoardRequire = 3
)

func FormatBoardRequire(i BoardRequire) string {
	switch i {
	case RequireNever:
		return "Never"
	case RequireThreads:
		return "Threads"
	case RequireReplies:
		return "Replies"
	case RequireAll:
		return "All posts"
	default:
		return "Unknown"
	}
}

type Board struct {
	ID            int
	Dir           string
	Name          string
	Description   string
	Type          BoardType
	Hide          BoardHide
	Lock          BoardLock
	Approval      BoardApproval
	Reports       bool
	Style         string
	Locale        string
	MinName       int
	MaxName       int
	MinEmail      int
	MaxEmail      int
	MinSubject    int
	MaxSubject    int
	MinMessage    int
	MaxMessage    int
	MinSizeThread int64
	MaxSizeThread int64
	MinSizeReply  int64
	MaxSizeReply  int64
	ThumbWidth    int
	ThumbHeight   int
	DefaultName   string
	WordBreak     int
	Truncate      int
	Threads       int
	Replies       int
	MaxThreads    int
	MaxReplies    int
	Oekaki        bool
	Backlinks     bool
	Files         int
	Instances     int
	Identifiers   BoardIdentifiers
	Gallery       bool
	Require       BoardRequire

	// Calculated fields.
	Uploads []string
	Embeds  []string
	Rules   []string
	Unique  int `diff:"-"`
}

const (
	DefaultBoardThreads     = 10
	DefaultBoardReplies     = 3
	DefaultBoardMaxName     = 75
	DefaultBoardMaxEmail    = 75
	DefaultBoardMaxSubject  = 75
	DefaultBoardMaxMessage  = 8000
	DefaultBoardWordBreak   = 200
	DefaultBoardDefaultName = "Anonymous"
	DefaultBoardTruncate    = 15
	DefaultBoardMaxSize     = 2097152
	DefaultBoardThumbWidth  = 250
	DefaultBoardThumbHeight = 250
	DefaultBoardFiles       = 1
	DefaultBoardInstances   = 1
	DefaultBoardGallery     = true
)

func NewBoard() *Board {
	return &Board{
		Threads:       DefaultBoardThreads,
		Replies:       DefaultBoardReplies,
		MaxName:       DefaultBoardMaxName,
		MaxEmail:      DefaultBoardMaxEmail,
		MaxSubject:    DefaultBoardMaxSubject,
		MaxMessage:    DefaultBoardMaxMessage,
		DefaultName:   DefaultBoardDefaultName,
		WordBreak:     DefaultBoardWordBreak,
		Truncate:      DefaultBoardTruncate,
		MaxSizeThread: DefaultBoardMaxSize,
		MaxSizeReply:  DefaultBoardMaxSize,
		ThumbWidth:    DefaultBoardThumbWidth,
		ThumbHeight:   DefaultBoardThumbHeight,
		Files:         DefaultBoardFiles,
		Instances:     DefaultBoardInstances,
		Gallery:       DefaultBoardGallery,
	}
}

func (b *Board) Validate() error {
	switch {
	case b.Dir != "" && !AlphaNumericAndSymbols.MatchString(b.Dir):
		return fmt.Errorf("dir must only consist of letters, numbers, hyphens and underscores")
	case strings.TrimSpace(b.Name) == "":
		return fmt.Errorf("name must be set")
	case b.MinName > b.MaxName:
		return fmt.Errorf("minimum %[1]s must be less than or equal to maximum %[1]s", "name length")
	case b.MinEmail > b.MaxEmail:
		return fmt.Errorf("minimum %[1]s must be less than or equal to maximum %[1]s", "email length")
	case b.MinSubject > b.MaxSubject:
		return fmt.Errorf("minimum %[1]s must be less than or equal to maximum %[1]s", "subject length")
	case b.MinMessage > b.MaxMessage:
		return fmt.Errorf("minimum %[1]s must be less than or equal to maximum %[1]s", "message length")
	case b.MinSizeThread > b.MaxSizeThread:
		return fmt.Errorf("minimum %[1]s must be less than or equal to maximum %[1]s", "thread file size")
	case b.MinSizeReply > b.MaxSizeReply:
		return fmt.Errorf("minimum %[1]s must be less than or equal to maximum %[1]s", "reply file size")
	}
	reservedDirs := []string{"captcha", "static", "sriracha", "sriracha_all"}
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

func (b *Board) MaxSizeLabel(thread bool) string {
	if thread {
		return FormatFileSize(b.MaxSizeThread)
	}
	return FormatFileSize(b.MaxSizeReply)
}

func (b *Board) HasUpload(mimeType string) bool {
	if len(b.Uploads) == 0 {
		return false
	}
	for _, upload := range b.Uploads {
		if upload == mimeType {
			return true
		}
	}
	return false
}

func (b *Board) HasEmbed(name string) bool {
	if len(b.Embeds) == 0 {
		return false
	}
	for _, embed := range b.Embeds {
		if embed == name {
			return true
		}
	}
	return false
}

func (b *Board) UploadTypesLabel(uploadTypes []*UploadType, account *Account) string {
	if len(b.Uploads) == 0 {
		return ""
	}
	var types []string
	found := make(map[string]bool)
	for _, u := range uploadTypes {
		if b.HasUpload(u.MIME) && !found[u.Ext] {
			found[u.Ext] = true
			types = append(types, strings.ToUpper(u.Ext))
		}
	}
	buf := &strings.Builder{}
	for i, t := range types {
		if i > 0 {
			if i == len(types)-1 {
				buf.WriteString(" " + G(b, account, "and") + " ")
			} else {
				buf.WriteString(", ")
			}
		}
		buf.WriteString(t)
	}
	return buf.String()
}
