package sriracha

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var reflinkPattern = regexp.MustCompile(`&gt;&gt;([0-9]+)`)

func (s *Server) servePost(db *Database, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid request", http.StatusInternalServerError)
		return
	}

	boardDir := formString(r, "board")
	b := db.boardByDir(boardDir)
	if b == nil {
		data := s.buildData(db, w, r)
		data.ManageError("no board was specified")
		data.execute(w)
		return
	}

	now := time.Now().Unix()
	post := &Post{
		Board:     b,
		Timestamp: now,
		Bumped:    now,
		Moderated: 1,
	}

	err := post.loadForm(r, b, s.config.Root)
	if err != nil {
		s.deletePostFiles(post)

		data := s.buildData(db, w, r)
		data.ManageError(err.Error())
		data.execute(w)
		return
	}

	if post.Parent != 0 {
		parent := db.postByID(b, post.Parent)
		if parent == nil || parent.Parent != 0 {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.ManageError("invalid post parent")
			data.execute(w)
			return
		}
	}

	ip := r.RemoteAddr
	if ip != "" {
		post.IP = hashIP(ip)
	}

	password := formString(r, "password")
	if password != "" {
		post.Password = hashData(password)
	}

	var (
		rawHTML      bool
		staffPost    bool
		staffCapcode string
	)
	data := s.buildData(db, w, r)
	if data.Account != nil {
		staffPost = formString(r, "capcode") != ""
		if staffPost {
			capcode := formInt(r, "capcode")
			if capcode < 0 || capcode > 2 || (data.Account.Role == RoleMod && capcode == 2) {
				capcode = 0
			}
			switch capcode {
			case 1:
				staffCapcode = "Mod"
			case 2:
				staffCapcode = "Admin"
			}

			rawHTML = formBool(r, "raw")
			if rawHTML {
				post.Message = html.UnescapeString(post.Message)
			}
		}
	}

	if !staffPost {
		for _, keyword := range db.allKeywords() {
			rgxp, err := regexp.Compile(keyword.Text)
			if err != nil {
				s.deletePostFiles(post)
				log.Fatalf("failed to compile regexp %s: %s", keyword.Text, err)
			}
			if rgxp.MatchString(post.Name) || rgxp.MatchString(post.Email) || rgxp.MatchString(post.Subject) || rgxp.MatchString(post.Message) {
				var action string
				var banExpire int64
				switch keyword.Action {
				case "hide":
					action = "hide"
				case "report":
					action = "report"
				case "delete":
					action = "delete"
				case "ban1h":
					action = "ban"
					banExpire = time.Now().Add(1 * time.Hour).Unix()
				case "ban1d":
					action = "ban"
					banExpire = time.Now().Add(24 * time.Hour).Unix()
				case "ban2d":
					action = "ban"
					banExpire = time.Now().Add(2 * 24 * time.Hour).Unix()
				case "ban1w":
					action = "ban"
					banExpire = time.Now().Add(7 * 24 * time.Hour).Unix()
				case "ban2w":
					action = "ban"
					banExpire = time.Now().Add(14 * 24 * time.Hour).Unix()
				case "ban1m":
					action = "ban"
					banExpire = time.Now().Add(28 * 24 * time.Hour).Unix()
				case "ban0":
					action = "ban"
				default:
					s.deletePostFiles(post)
					log.Fatalf("unknown keyword action: %s", keyword.Action)
				}

				switch action {
				case "hide":
					post.Moderated = 0
				case "report":
					// TODO add report
				case "ban":
					existing := db.banByIP(post.IP)
					if existing == nil {
						ban := &Ban{
							IP:        post.IP,
							Timestamp: time.Now().Unix(),
							Expire:    banExpire,
							Reason:    "Detected banned keyword.",
						}
						db.addBan(ban)

						db.log(nil, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info()+fmt.Sprintf(" Detected >>/keyword/%d", keyword.ID))
					}
				}

				if action == "delete" || action == "ban" {
					s.deletePostFiles(post)

					data := s.buildData(db, w, r)
					data.ManageError("Banned keyword detected in post.")
					data.execute(w)
					return
				}
			}
		}
	}

	if !rawHTML {
		for _, postHandler := range allPluginPostHandlers {
			err := postHandler(db, post)
			if err != nil {
				s.deletePostFiles(post)

				data := s.buildData(db, w, r)
				data.ManageError(err.Error())
				data.execute(w)
				log.Printf("warning: plugin failed to handle post event: %s", err.Error())
				return
			}
		}

		post.Message = reflinkPattern.ReplaceAllStringFunc(post.Message, func(s string) string {
			postID, err := strconv.Atoi(s[8:])
			if err != nil || postID <= 0 {
				return s
			}
			refPost := db.postByID(post.Board, postID)
			if refPost == nil {
				return s
			}
			className := "refop"
			if refPost.Parent != 0 {
				className = "refreply"
			}
			return fmt.Sprintf(`<a href="%sres/%d.html#%d" class="%s">%s</a>`, refPost.Board.Path(), refPost.Thread(), refPost.ID, className, s)
		})
	}

	if strings.TrimSpace(post.Message) == "" && post.File == "" {
		data := s.buildData(db, w, r)
		data.ManageError("Please upload a file and/or enter a message.")
		data.execute(w)
		return
	}

	if !rawHTML {
		post.Message = strings.ReplaceAll(post.Message, "\n", "<br>\n")
	}

	post.setNameBlock(b.DefaultName, staffCapcode)

	if !staffPost && (b.Approval == ApprovalAll || (b.Approval == ApprovalFile && post.File != "")) {
		post.Moderated = 0
	}

	db.addPost(post)

	if post.Parent != 0 && strings.ToLower(post.Email) != "sage" {
		// TODO check reply limit
		db.bumpThread(post.Parent, now)
	}

	s.rebuildThread(db, b, post)

	redir := fmt.Sprintf("%sres/%d.html#%d", b.Path(), post.Thread(), post.ID)
	http.Redirect(w, r, redir, http.StatusFound)
}
