package server

import (
	"log"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/leonelquinteros/gotext"
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

	var references []int
	for _, m := range RefLinkPattern.FindAllStringSubmatch(p.Message, -1) {
		if len(m) != 2 {
			continue
		}
		id, err := strconv.Atoi(m[1])
		if err == nil && !slices.Contains(references, id) {
			references = append(references, id)
		}
	}

	notified := make(map[string]bool)
	for _, referenceID := range references {
		reference := db.PostByID(referenceID)
		if reference == nil {
			continue
		}
		subs := db.SubscriptionsByPost(reference, true, false)
		for _, sub := range subs {
			if notified[sub.Email] {
				continue
			}
			n := notification{
				subscriptionID: sub.ID,
				postID:         p.ID,
				mentioned:      true,
			}
			s.notifications = append(s.notifications, n)
			notified[sub.Email] = true
		}
	}

	subs := db.SubscriptionsByPost(p, true, true)
	for _, sub := range subs {
		if notified[sub.Email] {
			continue
		}
		n := notification{
			subscriptionID: sub.ID,
			postID:         p.ID,
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

	type notificationInfo struct {
		n notification
		p *Post
	}

	var keep []notification
	var modified bool
	postCache := make(map[int]*Post)
	pending := make(map[string][]*notificationInfo)
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
		pending[sub.Email] = append(pending[sub.Email], &notificationInfo{n: n, p: post})
	}
	if modified {
		client, err := s.connectToMailServer()
		if err != nil {
			log.Fatalf("failed to send notifications: %s", err)
		}
		const batchSize = 16
		var sent int
		for email, allInfo := range pending {
			sort.Slice(allInfo, func(i, j int) bool {
				if allInfo[i].n.mentioned != allInfo[j].n.mentioned {
					return allInfo[i].n.mentioned
				} else if allInfo[i].p.Board.ID != allInfo[j].p.Board.ID {
					return allInfo[i].p.Board.Name < allInfo[j].p.Board.Name
				}
				return allInfo[i].p.ID < allInfo[j].p.ID
			})

			var message strings.Builder
			var lastBoard int
			var i int
			var mentioned bool
			var lastMentioned bool
			for _, info := range allInfo {
				if lastMentioned && !info.n.mentioned {
					message.WriteString("\n\n===\n\n")
					lastBoard = 0
				}

				p := info.p
				if p.Board.ID != lastBoard {
					if lastBoard != 0 {
						message.WriteString("\n\n")
					}
					message.WriteString(p.Board.Path())

					i = 0
					lastBoard = p.Board.ID
				}

				message.WriteString("\n" + string(p.URL(s.opt.SiteHome)))
				if info.n.mentioned {
					message.WriteString(" ***")
					mentioned = true
				}
				i++
				lastMentioned = info.n.mentioned
			}

			key := md5Sum(s.hashData(md5Sum(email)))
			message.WriteString("\n\n--\n" + gotext.Get("Manage Subscriptions") + "\n" + s.opt.SiteHome + "sriracha/subscribe/?email=" + email + "&key=" + key)

			l := len(allInfo)
			subject := gotext.GetN("%d new post", "%d new posts", l, l)
			if mentioned {
				subject = gotext.Get("(Mentioned) %s", subject)
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
	defer s.notificationsWaitGroup.Done()

	mentionTicker := time.NewTicker(time.Duration(s.config.Mentions) * time.Minute)
	defaultTicker := time.NewTicker(time.Duration(s.config.Notifications) * time.Minute)
	for {
		select {
		case <-mentionTicker.C:
			s.sendNotifications(true)
		case <-defaultTicker.C:
			s.sendNotifications(false)
		case <-s.shutdownNotifications:
			s.sendNotifications(false)
			return
		}
	}
}
