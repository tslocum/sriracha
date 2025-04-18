package sriracha

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	switch k.Action {
	case "hide":
		return "Hide until approved"
	case "report":
		return "Report"
	case "delete":
		return "Delete"
	case "ban1h":
		return "Delete + ban for 1 hour"
	case "ban1d":
		return "Delete + ban for 1 day"
	case "ban2d":
		return "Delete + ban for 2 days"
	case "ban1w":
		return "Delete + ban for 1 week"
	case "ban2w":
		return "Delete + ban for 2 weeks"
	case "ban1m":
		return "Delete + ban for 1 month"
	case "ban0":
		return "Delete + ban permanently"
	default:
		return "Unknown"
	}
}
