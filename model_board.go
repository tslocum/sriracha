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
	LockThread BoardLock = 1
	LockPost   BoardLock = 2
	LockStaff  BoardLock = 3
)

func formatBoardLock(l BoardLock) string {
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
	ID            int
	Dir           string
	Name          string
	Description   string
	Type          BoardType
	Lock          BoardLock
	Approval      BoardApproval
	Reports       bool
	Style         string
	Locale        string
	Delay         int
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

	// Calculated fields.
	Uploads []string
	Embeds  []string
	Rules   []string
	Unique  int `diff:"-"`
}

const (
	defaultBoardThreads     = 10
	defaultBoardReplies     = 3
	defaultBoardMaxName     = 75
	defaultBoardMaxEmail    = 255
	defaultBoardMaxSubject  = 75
	defaultBoardMaxMessage  = 8000
	defaultBoardWordBreak   = 200
	defaultBoardDefaultName = "Anonymous"
	defaultBoardTruncate    = 15
	defaultBoardMaxSize     = 2097152
	defaultBoardThumbWidth  = 250
	defaultBoardThumbHeight = 250
)

func newBoard() *Board {
	return &Board{
		Threads:       defaultBoardThreads,
		Replies:       defaultBoardReplies,
		MaxName:       defaultBoardMaxName,
		MaxEmail:      defaultBoardMaxEmail,
		MaxSubject:    defaultBoardMaxSubject,
		MaxMessage:    defaultBoardMaxMessage,
		DefaultName:   defaultBoardDefaultName,
		WordBreak:     defaultBoardWordBreak,
		Truncate:      defaultBoardTruncate,
		MaxSizeThread: defaultBoardMaxSize,
		MaxSizeReply:  defaultBoardMaxSize,
		ThumbWidth:    defaultBoardThumbWidth,
		ThumbHeight:   defaultBoardThumbHeight,
	}
}

func (b *Board) loadForm(r *http.Request, availableUploads []*uploadType, availableEmbeds [][2]string) {
	b.Dir = formString(r, "dir")
	b.Name = formString(r, "name")
	b.Description = formString(r, "description")
	b.Type = formRange(r, "type", TypeImageboard, TypeForum)
	b.Lock = formRange(r, "lock", LockNone, LockStaff)
	b.Approval = formRange(r, "approval", ApprovalNone, ApprovalAll)
	b.Reports = formBool(r, "reports")
	b.Style = formString(r, "style")
	b.Locale = formString(r, "locale")
	b.Delay = formInt(r, "delay")
	b.MinName = formInt(r, "minname")
	b.MaxName = formInt(r, "maxname")
	b.MinEmail = formInt(r, "minemail")
	b.MaxEmail = formInt(r, "maxemail")
	b.MinSubject = formInt(r, "minsubject")
	b.MaxSubject = formInt(r, "maxsubject")
	b.MinMessage = formInt(r, "minmessage")
	b.MaxMessage = formInt(r, "maxmessage")
	b.MinSizeThread = formInt64(r, "minsizethread")
	b.MaxSizeThread = formInt64(r, "maxsizethread")
	b.MinSizeReply = formInt64(r, "minsizereply")
	b.MaxSizeReply = formInt64(r, "maxsizereply")
	b.ThumbWidth = formInt(r, "thumbwidth")
	b.ThumbHeight = formInt(r, "thumbheight")
	b.DefaultName = formString(r, "defaultname")
	b.WordBreak = formInt(r, "wordbreak")
	b.Truncate = formInt(r, "truncate")
	b.Threads = formInt(r, "threads")
	b.Replies = formInt(r, "replies")
	b.MaxThreads = formInt(r, "maxthreads")
	b.MaxReplies = formInt(r, "maxreplies")
	b.Oekaki = formBool(r, "oekaki")
	b.Rules = formMultiString(r, "rules")

	b.Uploads = nil
	uploads := r.Form["uploads"]
	for _, upload := range uploads {
		var found bool
		for _, u := range availableUploads {
			if u.MIME == upload {
				found = true
				break
			}
		}
		if found {
			b.Uploads = append(b.Uploads, upload)
		}
	}

	b.Embeds = nil
	embeds := r.Form["embeds"]
	for _, embed := range embeds {
		var found bool
		for _, info := range availableEmbeds {
			if info[0] == embed {
				found = true
				break
			}
		}
		if found {
			b.Embeds = append(b.Embeds, embed)
		}
	}
}

func (b *Board) validate() error {
	switch {
	case b.Dir != "" && !alphaNumericAndSymbols.MatchString(b.Dir):
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
		return formatFileSize(b.MaxSizeThread)
	}
	return formatFileSize(b.MaxSizeReply)
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

func (b *Board) UploadTypesLabel() string {
	if len(b.Uploads) == 0 {
		return ""
	}
	var types []string
	found := make(map[string]bool)
	for _, u := range srirachaServer.config.UploadTypes() {
		if b.HasUpload(u.MIME) && !found[u.Ext] {
			found[u.Ext] = true
			types = append(types, strings.ToUpper(u.Ext))
		}
	}
	buf := &strings.Builder{}
	for i, t := range types {
		if i > 0 {
			if i == len(types)-1 {
				buf.WriteString(" and ")
			} else {
				buf.WriteString(", ")
			}
		}
		buf.WriteString(t)
	}
	return buf.String()
}
