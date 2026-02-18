package server

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

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

func (s *Server) sendNotifications(onlyMentions bool) {
	if len(s.notifications) == 0 {
		return
	}
	db := s.begin()
	defer db.Commit()

	var keep []notification
	var modified bool
	postCache := make(map[int]*Post)
	pending := make(map[string][]*Post)
	for _, n := range s.notifications {
		sub := db.SubscriptionByID(n.subscriptionID)
		if sub == nil {
			modified = true
			continue
		} else if onlyMentions && !n.mentioned {
			keep = append(keep, n)
			continue
		}
		modified = true
		post, ok := postCache[n.postID]
		if !ok {
			post = db.PostByID(n.postID)
			postCache[n.postID] = post
		}
		if post == nil {
			continue
		}
		pending[sub.Email] = append(pending[sub.Email], post)
	}
	if modified {
		client, err := s.connectToMailServer()
		if err != nil {
			log.Fatalf("failed to send notifications: %s", err)
		}
		const batchSize = 16
		var sent int
		for email, posts := range pending {
			sort.Slice(posts, func(i, j int) bool {
				if posts[i].Board.ID != posts[j].Board.ID {
					return posts[i].Board.Name < posts[j].Board.Name
				}
				return posts[i].ID < posts[j].ID
			})
			l := len(posts)
			var plural string
			if l != 1 {
				plural = "s"
			}
			subject := fmt.Sprintf("%d new post%s", l, plural)

			var message strings.Builder
			var lastBoard int
			var i int
			for _, p := range posts {
				if i != 0 && lastBoard != 0 {
					message.WriteString(", ")
				}
				if p.Board.ID != lastBoard {
					if lastBoard != 0 {
						message.WriteString("\n")
					}
					message.WriteString(p.Board.Path())
					i = 0
				}
				message.WriteString(" " + string(p.RefLink()))
				i++
			}
			if sent == batchSize {
				client.Close()
				client, err = s.connectToMailServer()
				if err != nil {
					log.Fatalf("failed to send notifications: %s", err)
				}
				sent = 0
			}
			err := s.sendMail(client, email, subject, message.String())
			if err != nil {
				log.Fatalf("failed to send email: %s", err)
			}
			sent++
		}
		client.Close()

		s.notifications = keep
	}
}

func (s *Server) handleNotifications() {
	defaultDelay := 24 * time.Hour
	mentionDelay := 1 * time.Hour

	defaultTicker := time.NewTicker(defaultDelay)
	mentionTicker := time.NewTicker(mentionDelay)
	for {
		select {
		case <-defaultTicker.C:
			s.sendNotifications(false)
		case <-mentionTicker.C:
			s.sendNotifications(true)
		}
	}
}
