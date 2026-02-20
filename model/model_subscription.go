package model

import (
	"fmt"

	. "codeberg.org/tslocum/sriracha/util"
)

type SubscriptionType int

const (
	SubscriptionAll     SubscriptionType = 0  // Subscribe to all new posts.
	SubscriptionThreads SubscriptionType = -1 // Subscribe to new threads.
)

type Subscription struct {
	ID      int
	IP      string // For anti-spam protection. Cleared after 24 hours.
	Confirm int64  // Timestamp when confirmation request was zent, or zero once confirmed.
	Email   string // Email address.
	Board   int    // Only set when Target is a SubscriptionType.
	Target  int    // When greater than zero, Target is a post ID. Otherwise, Target is a SubscriptionType.
}

func (s *Subscription) Validate() error {
	email := ParseEmail(s.Email)
	if email == "" {
		return fmt.Errorf("invalid email address")
	} else if s.Board < 0 {
		return fmt.Errorf("invalid board %d", s.Board)
	} else if s.Target < int(SubscriptionThreads) {
		return fmt.Errorf("invalid subscription target %d", s.Target)
	}
	return nil
}
