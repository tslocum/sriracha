package model

import (
	"fmt"
	"slices"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/util"
)

type ThresholdEvent int

const (
	EventPost   ThresholdEvent = 0
	EventThread ThresholdEvent = 1
	EventReport ThresholdEvent = 2
)

func FormatThresholdEvent(e ThresholdEvent) string {
	switch e {
	case EventPost:
		return "Post"
	case EventThread:
		return "Thread"
	case EventReport:
		return "Report"
	default:
		return "Unknown"
	}
}

type Threshold struct {
	ID       int
	Everyone bool
	Amount   int
	Event    ThresholdEvent
	Anywhere bool
	Duration int
	Action   string
}

func (t *Threshold) Validate() error {
	switch {
	case t.Amount <= 0:
		return fmt.Errorf("invalid threshold amount")
	case t.Duration <= 0:
		return fmt.Errorf("invalid threshold duration")
	case !slices.Contains(AllActions, t.Action):
		return fmt.Errorf("action must be set")
	}
	return nil
}

func (t *Threshold) Label(account *Account) string {
	var everyoneLabel, eventLabel, anywhereLabel, durationLabel, actionLabel string
	if !t.Everyone {
		everyoneLabel = G(nil, account, "an individual visitor")
	} else {
		everyoneLabel = G(nil, account, "Everyone")
	}
	if t.Amount == 1 {
		switch t.Event {
		case EventPost:
			eventLabel = G(nil, account, "Post")
		case EventThread:
			eventLabel = G(nil, account, "Thread")
		default:
			eventLabel = G(nil, account, "Report")
		}
	} else {
		switch t.Event {
		case EventPost:
			eventLabel = G(nil, account, "Posts")
		case EventThread:
			eventLabel = G(nil, account, "Threads")
		default:
			eventLabel = G(nil, account, "Reports")
		}
	}
	if !t.Anywhere {
		anywhereLabel = G(nil, account, "in an individual board")
	} else {
		anywhereLabel = G(nil, account, "Anywhere")
	}
	durationLabel = FormatDuration(time.Duration(t.Duration) * time.Second)
	actionLabel = G(nil, account, FormatAction(t.Action))
	return Get(nil, account, "When %[1]s adds more than %[2]d %[3]s %[4]s within %[5]s, %[6]s.", strings.ToLower(everyoneLabel), t.Amount, strings.ToLower(eventLabel), strings.ToLower(anywhereLabel), durationLabel, strings.ToLower(actionLabel))
}
