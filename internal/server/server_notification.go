package server

import (
	"fmt"
	"strings"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
)

// notification represents a pending notification. The referenced subscription
// and post are refreshed before the sending the notification.
type notification struct {
	subscriptionID int
	postID         int
	mentioned      bool
}

func (s *Server) queueNotifications(db *database.DB, p *Post) {
	if s.config.MailAddress == "" {
		return
	}

	subs := db.SubscriptionsByPost(p)
	for _, sub := range subs {
		mentioned := sub.Target > 0 && (p.Parent == sub.Target || strings.Contains(p.Message, fmt.Sprintf(`.html#%d">&gt;&gt;%d</a>`, sub.Target, sub.Target)))
		n := notification{
			subscriptionID: sub.ID,
			postID:         p.ID,
			mentioned:      mentioned,
		}
		s.notifications = append(s.notifications, n)
	}
}
