package sriracha

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/leonelquinteros/gotext"
)

type Keyword struct {
	ID     int
	Text   string
	Action string
	Boards []*Board
}

func (k *Keyword) validate() error {
	switch {
	case strings.TrimSpace(k.Text) == "":
		return fmt.Errorf("text must be set")
	case strings.TrimSpace(k.Action) == "":
		return fmt.Errorf("action must be set")
	}
	return nil
}

func (k *Keyword) HasBoard(id int) bool {
	if len(k.Boards) == 0 {
		return true
	}
	for _, b := range k.Boards {
		if b.ID == id {
			return true
		}
	}
	return false
}

func (k *Keyword) loadForm(db *Database, r *http.Request) {
	k.Text = formString(r, "text")
	k.Action = formString(r, "action")
	k.Boards = nil
	boards := r.Form["boards"]
	for _, board := range boards {
		boardID, err := strconv.Atoi(board)
		if err != nil || boardID <= 0 {
			continue
		}
		b := db.boardByID(boardID)
		if b == nil {
			continue
		}
		k.Boards = append(k.Boards, b)
	}
}

func (k *Keyword) HasBoardOption(id int) bool {
	if len(k.Boards) == 0 {
		return false
	}
	for _, b := range k.Boards {
		if b.ID == id {
			return true
		}
	}
	return false
}

func (k *Keyword) ActionLabel() string {
	var label string
	switch k.Action {
	case "hide":
		label = "Hide until approved"
	case "report":
		label = "Report"
	case "delete":
		label = "Delete"
	case "ban1h":
		label = "Delete & ban for 1 hour"
	case "ban1d":
		label = "Delete & ban for 1 day"
	case "ban2d":
		label = "Delete & ban for 2 days"
	case "ban1w":
		label = "Delete & ban for 1 week"
	case "ban2w":
		label = "Delete & ban for 2 weeks"
	case "ban1m":
		label = "Delete & ban for 1 month"
	case "ban0":
		label = "Delete & ban permanently"
	default:
		label = "Unknown"
	}
	return gotext.Get(label)
}
