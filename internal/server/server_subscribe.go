package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/leonelquinteros/gotext"
)

func (s *Server) subscriptionConfirmKey(sub *Subscription) string {
	return md5Sum(s.hashData(md5Sum(fmt.Sprintf("%s/%d", sub.Email, sub.Confirm))))
}

func (s *Server) serveSubscribe(db *database.DB, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)
	data.Boards = db.AllBoards()
	if !s.opt.Notifications {
		data.BoardError(w, "Email notifications are disabled.")
		return
	}
	data.Template = "subscribe"

	key := r.URL.Query().Get("key")
	if key != "" {
		email := r.URL.Query().Get("email")
		if email == "" {
			data.BoardError(w, "Invalid email.")
			return
		}
		expectedKey := md5Sum(s.hashData(md5Sum(email)))
		if key != expectedKey {
			data.BoardError(w, "Invalid access key.")
			return
		}
		data.Extra = email
		data.Extra2 = key

		var confirmed bool
		subs := db.SubscriptionsByEmail(email)
		for _, sub := range subs {
			if sub.Confirm == 0 {
				confirmed = true
				break
			}
		}
		if !confirmed {
			if len(subs) == 0 {
				data.BoardError(w, "Your email address is unconfirmed. Subscribe to request a confirmation link.")
				return
			}
			const errorMessage = "Click the confirmation link emailed to you."
			confirmKey := r.URL.Query().Get("confirm")
			if confirmKey == "" {
				data.BoardError(w, "Your email address is unconfirmed. "+errorMessage)
				return
			}
			expectedConfirmKey := s.subscriptionConfirmKey(subs[0])
			if confirmKey != expectedConfirmKey {
				data.BoardError(w, "Invalid confirmation key. "+errorMessage)
				return
			}
			subs[0].Confirm = 0
			subs[0].IP = ""
			db.UpdateSubscription(subs[0])

			data.Info = "Subscription confirmed."
		}

		if r.Method == http.MethodPost {
			for _, sub := range subs {
				v := FormNegInt(r, fmt.Sprintf("sub%d", sub.ID))

				// Board subscription.
				if sub.Board != 0 {
					switch v {
					case int(SubscriptionThreads), int(SubscriptionAll):
						sub.Target = v
						db.UpdateSubscription(sub)
					case 1:
						db.DeleteSubscription(sub)
					}
					continue
				}

				// Post subscription.
				if v == 1 {
					db.DeleteSubscription(sub)
				}
			}

			subs = db.SubscriptionsByEmail(email)
		}

		boardLabels := make(map[int]string)
		for _, board := range data.Boards {
			boardLabels[board.ID] = board.Path()
		}

		sort.Slice(subs, func(i, j int) bool {
			iBoard, jBoard := subs[i].Board != 0, subs[j].Board != 0
			if iBoard != jBoard {
				return iBoard
			} else if iBoard && jBoard {
				return boardLabels[subs[i].Board] < boardLabels[subs[j].Board]
			}
			return subs[i].Target < subs[j].Target
		})
		data.Subscriptions = subs

		data.execute(w)
		return
	}

	boardID := PathInt(r, "/sriracha/subscribe/board/")
	if boardID > 0 {
		board := db.BoardByID(boardID)
		if board == nil {
			data.BoardError(w, "Invalid board.")
			return
		}
		data.Board = board
	} else {
		postID := PathInt(r, "/sriracha/subscribe/post/")
		if postID > 0 {
			post := db.PostByID(postID)
			if post == nil {
				data.BoardError(w, "Invalid post.")
				return
			}
			data.Board = post.Board
			data.Post = post
		}
	}

	if data.Post == nil && data.Board == nil {
		data.Redirect(w, r, "/sriracha/")
		return
	}

	if r.Method == http.MethodPost {
		email := FormString(r, "email")
		if email == "" {
			data.BoardError(w, "Enter your email address to subscribe.")
			return
		}

		const confirmErrorMessage = "You already requested a confirmation link. You may request another confirmation link when 24 hours have passed."
		ipHash := s.hashIP(r)
		ipSub := db.SubscriptionByIP(ipHash)
		if ipSub != nil {
			data.BoardError(w, confirmErrorMessage)
			return
		}

		var confirmed bool
		subs := db.SubscriptionsByEmail(email)
		for _, sub := range subs {
			if sub.Confirm == 0 {
				confirmed = true
				break
			}
		}

		var confirmTime int64
		if !confirmed {
			if len(subs) != 0 {
				data.BoardError(w, confirmErrorMessage)
				return
			}

			if s.notificationsPattern != nil {
				var matched bool
				address := ParseEmail(email)
				if address != "" {
					atSymbol := strings.IndexRune(address, '@')
					if atSymbol != -1 {
						domain := address[atSymbol+1:]
						matched = s.notificationsPattern.MatchString(domain)
					}
				}
				if !matched {
					data.BoardError(w, "Sorry, only the following email address domains are allowed: "+s.config.MailDomains)
					return
				}
			}

			confirmTime = time.Now().Unix()
		}

		sub := &Subscription{
			Confirm: confirmTime,
			Email:   email,
		}
		if !confirmed {
			sub.IP = ipHash
		}
		if data.Post != nil {
			sub.Target = data.Post.ID
		} else {
			sub.Board = data.Board.ID
			sub.Target = int(FormRange(r, "notify", SubscriptionThreads, SubscriptionAll))
		}
		err := sub.Validate()
		if err != nil {
			data.BoardError(w, fmt.Sprintf("Failed to add subscription: %s", err))
			return
		}

		var target string
		if data.Post != nil {
			target = fmt.Sprintf("No.%d", data.Post.ID)
		} else {
			target = data.Board.Path()
		}
		if !confirmed {
			const errorMessage = "Failed to send confirmation link. Please try again later."
			client, err := s.connectToMailServer()
			if err != nil {
				data.BoardError(w, errorMessage)
				return
			}
			subject := gotext.Get("Subscribe to %s", target)
			key := md5Sum(s.hashData(md5Sum(email)))
			confirmKey := s.subscriptionConfirmKey(sub)
			message := s.opt.SiteHome + "sriracha/subscribe/?email=" + email + "&key=" + key + "&confirm=" + confirmKey
			err = s.sendMail(client, sub.Email, subject, message)
			client.Close()
			if err != nil {
				data.BoardError(w, errorMessage)
				return
			}
		}

		var updated bool
		if confirmed {
			for _, existing := range subs {
				if sub.Board == existing.Board && (sub.Board != 0 || sub.Target == existing.Target) {
					existing.Board = sub.Board
					existing.Target = sub.Target
					db.UpdateSubscription(existing)
					updated = true
					break
				}
			}
		}
		if !updated {
			db.AddSubscription(sub)
		}

		data.Template = "board_info"
		if !confirmed {
			data.Info = "Please confirm your subscription by clicking the link emailed to you."
		} else {
			data.Info = fmt.Sprintf("Subscribed to %s", target)
		}
	}

	data.execute(w)
}
