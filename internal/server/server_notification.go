package server

import (
	"log"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/tslocum/gotext"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

// mailBatchSize is the maximum number of emails to send using a single SMTP connection.
// This is required because some SMTP servers limit the number of messages that may
// be sent at once. When the batch size is reached, a fresh SMTP connection is obtained.
const mailBatchSize = 16

// notification represents a pending notification. The referenced subscription
// and post are refreshed before the sending the notification.
type notification struct {
	subscriptionID int
	postID         int
	mentioned      bool
}

func (s *Server) queueNotifications(db serverDB, p *Post) {
	if !s.opt.Notifications {
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
	s.lock.Lock()
	defer s.lock.Unlock()

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
	if !modified {
		return // No pending notifications.
	}

	client, err := s.connectToMailServer()
	if err != nil {
		log.Fatalf("failed to send notifications: %s", err)
	}
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

		// Reconnect when batch size is reached.
		if sent == mailBatchSize {
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

func (s *Server) queueStaffNotifications() {
	s.staffNotifications <- struct{}{}
}

func (s *Server) sendStaffNotifications() {
	s.lock.Lock()
	defer s.lock.Unlock()

	sum := func(m map[AccountNotification]int) int {
		var total int
		for _, v := range m {
			total += v
		}
		return total
	}
	size := make(map[AccountRole]int)
	var found bool
	for role := MinRole; role <= MaxActiveRole; role++ {
		size[role] = sum(s.queueSize[role])
		if !found && size[role] != 0 {
			found = true
		}
	}
	if !found {
		return
	}

	db := s.begin()
	defer db.Commit()

	now := time.Now().Unix()
	messages := make(map[string][]string)
	for _, a := range db.AllAccounts() {
		if a.Role == RoleDisabled || a.Email == "" || len(a.Notifications) == 0 || a.LastNotified != 0 || size[a.Role] == 0 {
			continue
		}
		for n := MinNotification; n <= MaxNotification; n++ {
			if s.queueSize[a.Role][n] > 0 && slices.Contains(a.Notifications, n) {
				switch n {
				case NotificationPending:
					messages[a.Email] = append(messages[a.Email], G(nil, a, "A new post is pending."))
				case NotificationReport:
					messages[a.Email] = append(messages[a.Email], G(nil, a, "A post has been reported."))
				case NotificationPrune:
					messages[a.Email] = append(messages[a.Email], G(nil, a, "A thread has been pruned."))
				case NotificationAppeal:
					messages[a.Email] = append(messages[a.Email], G(nil, a, "A ban appeal is pending."))
				}
			}
		}
		if len(messages[a.Email]) != 0 {
			db.UpdateAccountLastNotified(a.ID, now)
		}
	}
	if len(messages) == 0 {
		return
	}

	client, err := s.connectToMailServer()
	if err != nil {
		log.Fatalf("failed to send staff notifications: %s", err)
	}
	var sent int
	subject := gotext.Get("Moderation Request")
	for email, msgs := range messages {
		message := strings.Join(msgs, "\n\n") + "\n\n--\n" + gotext.Get("Manage Subscriptions") + "\n" + s.opt.SiteHome + "sriracha/preference/"

		// Reconnect when batch size is reached.
		if sent == mailBatchSize {
			client.Close()
			client, err = s.connectToMailServer()
			if err != nil {
				log.Fatalf("failed to send staff notifications: %s", err)
			}
			sent = 0
		}

		err := s.sendMail(client, email, subject, message)
		if err != nil {
			log.Fatalf("failed to send email: %s", err)
		}
		sent++
	}
	client.Close()
}

func (s *Server) handleStaffNotifications() {
	defer s.staffNotificationsWaitGroup.Done()

	const delay = 10 * time.Minute
	var shutdown bool
	for {
		if shutdown {
			return
		}

		select {
		case <-s.staffNotifications:
		case <-s.shutdownStaffNotifications:
			shutdown = true
		}

		if !shutdown {
			t := time.NewTimer(delay)
			var timeout bool
			for {
				select {
				case <-s.staffNotifications:
				case <-t.C:
					timeout = true
				case <-s.shutdownStaffNotifications:
					shutdown = true
				}
				if timeout || shutdown {
					break
				}
			}
		}

		s.sendStaffNotifications()
	}
}
