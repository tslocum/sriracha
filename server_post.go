package sriracha

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/leonelquinteros/gotext"
)

var (
	reflinkPattern = regexp.MustCompile(`&gt;&gt;([0-9]+)`)
	quotePattern   = regexp.MustCompile(`^&gt;(.*)$`)
	urlPattern     = regexp.MustCompile(`(?i)(((f|ht)tp(s)?:\/\/)[-a-zA-Zа-яА-Я()0-9@%\!_+.,~#?&;:|\'\/=]+)`)
	fixURLPattern1 = regexp.MustCompile(`(?i)\(\<a href\=\"(.*)\)"\ target\=\"\_blank\">(.*)\)\<\/a>`)
	fixURLPattern2 = regexp.MustCompile(`(?i)\<a href\=\"(.*)\."\ target\=\"\_blank\">(.*)\.\<\/a>`)
	fixURLPattern3 = regexp.MustCompile(`(?i)\<a href\=\"(.*)\,"\ target\=\"\_blank\">(.*)\,\<\/a>`)
)

type embedInfo struct {
	Title string `json:"title"`
	Thumb string `json:"thumbnail_url"`
	HTML  string `json:"html"`
}

func (s *Server) servePost(db *Database, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid request", http.StatusInternalServerError)
		return
	}

	boardDir := formString(r, "board")
	b := db.BoardByDir(boardDir)
	if b == nil {
		data := s.buildData(db, w, r)
		data.BoardError(w, gotext.Get("No board specified."))
		return
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
		}
	}

	switch b.Lock {
	case LockPost:
		if !staffPost {
			data := s.buildData(db, w, r)
			data.BoardError(w, gotext.Get("Board locked. No new posts may be created."))
			return
		}
	case LockStaff:
		data := s.buildData(db, w, r)
		data.BoardError(w, gotext.Get("Board locked. No new posts may be created."))
		return
	}

	now := time.Now().Unix()
	post := &Post{
		Board:     b,
		Timestamp: now,
		Bumped:    now,
		Moderated: 1,
	}

	post.IP = hashIP(r)

	if b.Delay != 0 {
		lastPost := db.lastPostByIP(post.Board, post.IP)
		if lastPost != nil {
			nextPost := lastPost.Timestamp + int64(b.Delay)
			if time.Now().Unix() < nextPost {
				waitTime := time.Until(time.Unix(nextPost, 0)) // This should be rounded to the nearest second. Oh well.
				data := s.buildData(db, w, r)
				data.BoardError(w, gotext.Get("Please wait %s before creating a new post.", waitTime))
				return
			}
		}
	}

	err := post.loadForm(r, s.config.Root, s.config.SaltTrip)
	if err != nil {
		s.deletePostFiles(post)

		data := s.buildData(db, w, r)
		data.BoardError(w, err.Error())
		return
	}

	var parentPost *Post
	if post.Parent != 0 {
		parentPost = db.PostByID(post.Parent)
		if parentPost == nil || parentPost.Parent != 0 {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, gotext.Get("No post selected."))
			return
		}
	}

	oekakiPost := b.Oekaki && formBool(r, "oekaki")
	skipCAPTCHA := oekakiPost && strings.HasSuffix(post.File, ".tgkr")

	if !staffPost {
		if b.Lock == LockThread && parentPost == nil {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, gotext.Get("You may only reply to threads."))
			return
		}
		if s.opt.CAPTCHA && !skipCAPTCHA {
			expired := db.expiredCAPTCHAs()
			for _, c := range expired {
				db.deleteCAPTCHA(c.IP)
				os.Remove(filepath.Join(s.config.Root, "captcha", c.Image+".png"))
			}

			var solved bool
			challenge := db.getCAPTCHA(post.IP)
			if challenge != nil {
				solution := formString(r, "captcha")
				if strings.ToLower(solution) == challenge.Text {
					solved = true
					db.deleteCAPTCHA(post.IP)
					os.Remove(filepath.Join(s.config.Root, "captcha", challenge.Image+".png"))
				}
			}
			if !solved {
				s.deletePostFiles(post)

				data := s.buildData(db, w, r)
				data.BoardError(w, gotext.Get("Incorrect CAPTCHA text. Please try again."))
				return
			}
		}
	}

	if oekakiPost && post.File == "" {
		data := s.buildData(db, w, r)
		data.Template = "oekaki"
		for key, values := range r.Form {
			if len(values) == 0 {
				continue
			}
			data.Message += template.HTML(fmt.Sprintf(`<input type="hidden" name="%s" value="%s">`+"\n", html.EscapeString(key), html.EscapeString(values[0])))
		}
		data.Message2 = template.HTML(`
		<script type="text/javascript">
		Tegaki.open({
			width: ` + strconv.Itoa(s.opt.OekakiWidth) + `,
			height: ` + strconv.Itoa(s.opt.OekakiHeight) + `,
			saveReplay: true,
			onDone: onDone,
			onCancel: onCancel
		});
		</script>`)
		data.execute(w)
		return
	}

	if post.File == "" && len(b.Embeds) > 0 {
		embed := formString(r, "embed")
		if embed != "" {
			for _, embedName := range b.Embeds {
				var embedURL string
				for _, info := range s.opt.Embeds {
					if info[0] == embedName {
						embedURL = info[1]
						break
					}
				}
				if embedURL == "" {
					continue
				}

				resp, err := http.Get(strings.ReplaceAll(embedURL, "SRIRACHA_EMBED", embed))
				if err != nil {
					continue
				}
				defer resp.Body.Close()

				info := &embedInfo{}
				err = json.NewDecoder(resp.Body).Decode(&info)
				if err != nil || info.Title == "" || info.Thumb == "" || info.HTML == "" || !strings.HasPrefix(info.Thumb, "https://") {
					continue
				}

				thumbResp, err := http.Get(info.Thumb)
				if err != nil {
					continue
				}
				defer thumbResp.Body.Close()

				buf, err := io.ReadAll(thumbResp.Body)
				if err != nil {
					continue
				}

				mimeType := mimetype.Detect(buf).String()

				fileExt := mimeToExt(mimeType)
				if fileExt == "" {
					continue
				}

				thumbName := fmt.Sprintf("%d.%s", time.Now().UnixNano(), fileExt)
				thumbPath := filepath.Join(s.config.Root, b.Dir, "thumb", thumbName)

				err = post.createThumbnail(buf, mimeType, true, thumbPath)
				if err != nil {
					continue
				}

				post.FileHash = "e " + embedName + " " + info.Title
				post.FileOriginal = embed
				post.File = info.HTML
				post.Thumb = thumbName
				break
			}

			if post.File == "" {
				data := s.buildData(db, w, r)
				data.BoardError(w, gotext.Get("Failed to embed media."))
				return
			}
		}
	}

	if post.FileHash != "" {
		existing := db.PostByFileHash(post.FileHash)
		if existing != nil {
			var postLink string
			if existing.Moderated != ModeratedHidden {
				postLink = fmt.Sprintf(` <a href="%sres/%d.html#%d">here</a>`, existing.Board.Path(), existing.Thread(), existing.ID)
			}

			var uploadType = "file"
			if post.IsEmbed() {
				uploadType = "embed"
			}

			data := s.buildData(db, w, r)
			data.Template = "board_error"
			data.Info = fmt.Sprintf("Duplicate %s uploaded.", uploadType)
			data.Message = template.HTML(fmt.Sprintf(`<div style="text-align: center;">That %s has already been posted%s.</div><br>`, uploadType, postLink))
			data.execute(w)
			return
		}
	}

	if rawHTML {
		post.Message = html.UnescapeString(post.Message)
	}

	var addReport bool
	if !staffPost {
		if parentPost != nil && parentPost.Locked {
			data := s.buildData(db, w, r)
			data.BoardError(w, gotext.Get("That thread is locked."))
			return
		}

		for _, keyword := range db.allKeywords() {
			if !keyword.HasBoard(b.ID) {
				continue
			}
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
					addReport = true
				case "ban":
					existing := db.banByIP(post.IP)
					if existing == nil {
						ban := &Ban{
							IP:        post.IP,
							Timestamp: time.Now().Unix(),
							Expire:    banExpire,
							Reason:    gotext.Get("Detected banned keyword."),
						}
						db.addBan(ban)

						db.log(nil, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info()+fmt.Sprintf(" Detected >>/keyword/%d", keyword.ID))
					}
				}

				if action == "delete" || action == "ban" {
					s.deletePostFiles(post)

					data := s.buildData(db, w, r)
					data.BoardError(w, gotext.Get("Detected banned keyword."))
					return
				}
			}
		}
	}

	if !rawHTML {
		if post.Board.WordBreak != 0 {
			pattern, err := regexp.Compile(`[^\s]{` + strconv.Itoa(post.Board.WordBreak) + `,}`)
			if err != nil {
				log.Fatal(err)
			}

			buf := &strings.Builder{}
			post.Message = pattern.ReplaceAllStringFunc(post.Message, func(s string) string {
				buf.Reset()
				for i, r := range s {
					if i != 0 && i%post.Board.WordBreak == 0 {
						buf.WriteRune('\n')
					}
					buf.WriteRune(r)
				}
				return buf.String()
			})
		}

		for _, info := range allPluginPostHandlers {
			db.plugin = info.Name
			err := info.Handler(db, post)
			if err != nil {
				s.deletePostFiles(post)

				if _, ok := err.(*HTMLError); ok {
					w.Write([]byte(err.Error()))
				} else {
					data := s.buildData(db, w, r)
					data.BoardError(w, err.Error())
				}
				return
			}
			post.Message = strings.ReplaceAll(post.Message, "<br>", "\n")
		}
		db.plugin = ""

		var foundURL bool
		post.Message = urlPattern.ReplaceAllStringFunc(post.Message, func(s string) string {
			foundURL = true
			match := urlPattern.FindStringSubmatch(post.Message)
			return fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, match[1], match[1])
		})
		if foundURL {
			post.Message = fixURLPattern1.ReplaceAllString(post.Message, `(<a href="$1" target="_blank">$2</a>)`)
			post.Message = fixURLPattern2.ReplaceAllString(post.Message, `<a href="$1" target="_blank">$2</a>.`)
			post.Message = fixURLPattern3.ReplaceAllString(post.Message, `<a href="$1" target="_blank">$2</a>,`)
		}

		post.Message = reflinkPattern.ReplaceAllStringFunc(post.Message, func(s string) string {
			postID, err := strconv.Atoi(s[8:])
			if err != nil || postID <= 0 {
				return s
			}
			refPost := db.PostByID(postID)
			if refPost == nil {
				return s
			}
			className := "refop"
			if refPost.Parent != 0 {
				className = "refreply"
			}
			return fmt.Sprintf(`<a href="%sres/%d.html#%d" class="%s">%s</a>`, refPost.Board.Path(), refPost.Thread(), refPost.ID, className, s)
		})

		var quote bool
		lines := strings.Split(post.Message, "\n")
		for i := range lines {
			lines[i] = quotePattern.ReplaceAllStringFunc(lines[i], func(s string) string {
				quote = true
				return `<span class="unkfunc">` + s + `</span>`
			})
		}
		if quote {
			post.Message = strings.Join(lines, "\n")
		}
	}

	if strings.TrimSpace(post.Message) == "" && post.File == "" {
		maxSize := post.Board.MaxSizeThread
		if post.Parent != 0 {
			maxSize = post.Board.MaxSizeReply
		}
		var options []string
		if maxSize != 0 {
			options = append(options, "upload a file")
		}
		if len(post.Board.Embeds) != 0 {
			options = append(options, "enter an embed URL")
		}
		if post.Board.MaxMessage != 0 {
			options = append(options, "enter a message")
		}
		buf := &strings.Builder{}
		for i, o := range options {
			if i > 0 {
				if i == len(options)-1 {
					buf.WriteString(" or ")
				} else {
					buf.WriteString(", ")
				}
			}
			buf.WriteString(o)
		}
		data := s.buildData(db, w, r)
		data.BoardError(w, fmt.Sprintf("Please %s.", buf.String()))
		return
	}

	post.setNameBlock(b.DefaultName, staffCapcode)

	if !rawHTML {
		post.Message = strings.ReplaceAll(post.Message, "\n", "<br>\n")
	}

	if post.Password != "" {
		post.Password = hashData(post.Password)
	}

	if !staffPost && (b.Approval == ApprovalAll || (b.Approval == ApprovalFile && post.File != "")) {
		post.Moderated = 0
	}

	postCopy := post.Copy()
	for _, info := range allPluginInsertHandlers {
		db.plugin = info.Name
		err := info.Handler(db, postCopy)
		if err != nil {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, err.Error())
			return
		}
	}
	db.plugin = ""

	db.addPost(post)

	if post.Moderated == ModeratedHidden {
		data.Template = "board_info"
		data.Info = gotext.Get("Your post will be shown once it has been approved.")
		data.execute(w)
		return
	} else if addReport {
		report := &Report{
			Board:     b,
			Post:      post,
			Timestamp: time.Now().Unix(),
			IP:        hashIP(r),
		}
		db.addReport(report)
	}

	if post.Parent == 0 {
		for _, thread := range db.trimThreads(post.Board) {
			s.deletePost(db, thread)
		}
	} else if strings.ToLower(post.Email) != "sage" {
		bump := post.Board.MaxReplies == 0 || db.replyCount(post.Parent) <= post.Board.MaxReplies
		if bump {
			db.bumpThread(post.Parent, now)
		}
	}

	s.rebuildThread(db, post)

	redir := fmt.Sprintf("%sres/%d.html#%d", b.Path(), post.Thread(), post.ID)
	http.Redirect(w, r, redir, http.StatusFound)
}
