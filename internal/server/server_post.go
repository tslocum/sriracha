package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/aquilax/tripcode"
	"github.com/gabriel-vasile/mimetype"
	"github.com/nfnt/resize"
)

var postUploadFileLock = &sync.Mutex{}

type embedInfo struct {
	Title string `json:"title"`
	Thumb string `json:"thumbnail_url"`
	HTML  string `json:"html"`
}

func resizeImage(b *Board, r io.Reader, mimeType string) (image.Image, error) {
	var img image.Image
	var err error
	switch mimeType {
	case "image/jpeg", "image/pjpeg":
		img, err = jpeg.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("unsupported filetype")
		}
	case "image/gif":
		img, err = gif.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("unsupported filetype")
		}
	case "image/png":
		img, err = png.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("unsupported filetype")
		}
	}
	return resize.Thumbnail(uint(b.ThumbWidth), uint(b.ThumbHeight), img, resize.Lanczos3), nil
}

func writeImage(img image.Image, mimeType string, filePath string) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	switch mimeType {
	case "image/jpeg":
		err = jpeg.Encode(file, img, nil)
		if err != nil {
			return fmt.Errorf("unsupported filetype")
		}
	case "image/gif":
		err = gif.Encode(file, img, nil)
		if err != nil {
			return fmt.Errorf("unsupported filetype")
		}
	case "image/png":
		err = png.Encode(file, img)
		if err != nil {
			return fmt.Errorf("unsupported filetype")
		}
	}
	return nil
}

func createPostThumbnail(p *Post, buf []byte, mimeType string, mediaOverlay bool, thumbPath string) error {
	thumbImg, err := resizeImage(p.Board, bytes.NewReader(buf), mimeType)
	if err != nil {
		return err
	}

	if mediaOverlay {
		thumbImg = p.AddMediaOverlay(thumbImg)
	}

	bounds := thumbImg.Bounds()
	p.ThumbWidth, p.ThumbHeight = bounds.Dx(), bounds.Dy()

	err = writeImage(thumbImg, mimeType, thumbPath)
	if err != nil {
		return fmt.Errorf("unsupported filetype")
	}
	return nil
}

func setFileAndThumb(p *Post, fileExt string, thumbExt string) {
	postUploadFileLock.Lock()
	defer postUploadFileLock.Unlock()

	fileID := time.Now().UnixNano()
	fileIDString := fmt.Sprintf("%d", fileID)

	if thumbExt == "" {
		switch fileExt {
		case "jpg", "png", "gif":
			thumbExt = fileExt
		case "svg":
			thumbExt = "png"
		default:
			thumbExt = "jpg"
		}
	}

	p.File = fileIDString + "." + fileExt
	p.Thumb = fileIDString + "s." + thumbExt
}

func setPostFileAttributes(p *Post, buf []byte) error {
	p.FileHash = calculateFileHash(buf)

	p.FileSize = int64(len(buf))
	return nil
}

func (s *Server) loadPostForm(db *database.DB, r *http.Request, p *Post) error {
	limitString := func(v string, limit int) string {
		if len(v) > limit {
			return v[:limit]
		}
		return v
	}

	p.Parent = FormInt(r, "parent")
	p.Password = FormString(r, "password")

	p.Name = limitString(FormString(r, "name"), p.Board.MaxName)
	p.Email = limitString(FormString(r, "email"), p.Board.MaxEmail)
	p.Subject = limitString(FormString(r, "subject"), p.Board.MaxSubject)
	p.Message = html.EscapeString(limitString(FormString(r, "message"), p.Board.MaxMessage))

	if len(p.Name) < p.Board.MinName {
		if p.Board.MinName == 1 {
			return fmt.Errorf("please enter a name")
		} else {
			return fmt.Errorf("name too short: must be at least %d characters in length", p.Board.MinName)
		}
	}
	if len(p.Email) < p.Board.MinEmail {
		if p.Board.MinEmail == 1 {
			return fmt.Errorf("please enter an email")
		} else {
			return fmt.Errorf("email too short: must be at least %d characters in length", p.Board.MinEmail)
		}
	}
	if len(p.Subject) < p.Board.MinSubject && (p.Board.Type == TypeImageboard || p.Parent == 0) {
		if p.Board.MinSubject == 1 {
			return fmt.Errorf("please enter a subject")
		} else {
			return fmt.Errorf("subject too short: must be at least %d characters in length", p.Board.MinSubject)
		}
	}
	if len(p.Message) < p.Board.MinMessage {
		if p.Board.MinMessage == 1 {
			return fmt.Errorf("please enter a message")
		} else {
			return fmt.Errorf("message too short: must be at least %d characters in length", p.Board.MinMessage)
		}
	}

	if strings.ContainsRune(p.Name, '#') {
		split := strings.SplitN(p.Name, "#", 3)

		p.Name = split[0]
		standardPass := split[1]
		var securePass string
		if len(split) == 3 {
			securePass = split[2]
		}

		if standardPass != "" {
			p.Tripcode = tripcode.Tripcode(standardPass)
		}
		if securePass != "" {
			if standardPass != "" {
				p.Tripcode += "!"
			}
			p.Tripcode += "!" + tripcode.SecureTripcode(securePass, s.config.SaltTrip)
		}
	}

	if p.Parent != 0 && p.Board.Type == TypeForum {
		p.Subject = ""
	}
	return nil
}

func (s *Server) loadPostFiles(r *http.Request, p *Post) ([]*multipart.FileHeader, error) {
	if r.PostForm == nil {
		const maxMemory = 32 << 20 // 32 MB
		err := r.ParseMultipartForm(maxMemory)
		if err != nil {
			return nil, err
		}
	}
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, nil
	}
	files := r.MultipartForm.File["file"]
	if len(files) > p.Board.Files {
		if p.Board.Files == 0 {
			return nil, fmt.Errorf("file attachments are not allowed")
		}
		return nil, fmt.Errorf("too many files: only %d files may be uploaded at once", p.Board.Files)
	}
	return files, nil
}

func (s *Server) loadPostFile(db *database.DB, r *http.Request, p *Post, fileHeader *multipart.FileHeader) error {
	minSize := p.Board.MinSizeThread
	maxSize := p.Board.MaxSizeThread
	if p.Parent != 0 {
		minSize = p.Board.MinSizeReply
		maxSize = p.Board.MaxSizeReply
	}

	if maxSize == 0 {
		return nil
	} else if minSize > 0 && fileHeader.Size < minSize {
		if minSize == 1 {
			if len(p.Board.Embeds) == 0 {
				return fmt.Errorf("a file is required")
			}
			return fmt.Errorf("a file or embed is required")
		}
		return fmt.Errorf("a file %s or larger is required", FormatFileSize(minSize))
	} else if fileHeader.Size > maxSize {
		return fmt.Errorf("file too large: maximum file size allowed is %s", FormatFileSize(maxSize))
	}

	formFile, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer formFile.Close()

	buf, err := io.ReadAll(formFile)
	if err != nil {
		return err
	}

	if int64(len(buf)) < minSize {
		if minSize == 1 {
			if len(p.Board.Embeds) == 0 {
				return fmt.Errorf("a file is required")
			} else {
				return fmt.Errorf("a file or embed is required")
			}
		} else {
			return fmt.Errorf("a file %s or larger is required", FormatFileSize(minSize))
		}
	} else if int64(len(buf)) > maxSize {
		return fmt.Errorf("that file exceeds the maximum file size: %s", FormatFileSize(maxSize))
	}

	p.FileMIME = mimetype.Detect(buf).String()
	p.FileOriginal = fileHeader.Filename

	oekakiPost := p.Board.Oekaki && p.FileMIME == "application/octet-stream" && len(buf) >= 3 && buf[0] == 0x54 && buf[1] == 0x47 && buf[2] == 0x4B
	if oekakiPost {
		p.FileMIME = "application/x-tegaki"
	}

	var fileExt string
	var fileThumb string
	if p.Board.HasUpload(p.FileMIME) {
		for _, u := range s.config.UploadTypes() {
			if u.MIME == p.FileMIME {
				fileExt = u.Ext
				fileThumb = u.Thumb
				break
			}
		}
	}
	if fileExt == "" {
		if oekakiPost {
			fileExt = "tgkr"
		} else {
			for _, info := range allPluginAttachHandlers {
				db.Plugin = info.Name
				handled, err := info.Handler(db, p, buf)
				if err != nil {
					db.Plugin = ""
					return err
				} else if handled {
					db.Plugin = ""
					return nil
				}
			}
			db.Plugin = ""
			return fmt.Errorf("unsupported filetype")
		}
	}

	var thumbExt string
	var thumbData []byte
	if fileThumb != "" && fileThumb != "none" {
		thumbData, err = os.ReadFile("static/img/" + fileThumb)
		if err != nil {
			log.Fatalf("failed to open thumbnail file %s: %s", fileThumb, err)
		}

		thumbExt = MIMEToExt(mimetype.Detect(thumbData).String())
	}

	setFileAndThumb(p, fileExt, thumbExt)

	err = setPostFileAttributes(p, buf)
	if err != nil {
		return err
	}
	if oekakiPost && FormBool(r, "oekaki") {
		p.FileOriginal = FormString(r, "title")
	}

	srcPath := filepath.Join(s.config.Root, p.Board.Dir, "src", p.File)
	thumbPath := filepath.Join(s.config.Root, p.Board.Dir, "thumb", p.Thumb)

	err = os.WriteFile(srcPath, buf, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	if oekakiPost {
		formThumb, formThumbHeader, err := r.FormFile("thumb")
		if err != nil || formThumbHeader == nil || formThumbHeader.Size < minSize {
			return fmt.Errorf("a thumbnail is required")
		}

		buf, err := io.ReadAll(formThumb)
		if err != nil {
			log.Fatal(err)
		}

		imgConfig, _, err := image.DecodeConfig(bytes.NewReader(buf))
		if err != nil {
			return fmt.Errorf("unsupported thumbnail filetype")
		}
		p.FileWidth, p.FileHeight = imgConfig.Width, imgConfig.Height

		return createPostThumbnail(p, buf, "image/png", false, thumbPath)
	}

	if fileThumb == "none" {
		p.Thumb = ""
		return nil
	} else if fileThumb != "" {
		return createPostThumbnail(p, thumbData, mimetype.Detect(thumbData).String(), false, thumbPath)
	}

	isImage := p.FileMIME == "image/jpeg" || p.FileMIME == "image/pjpeg" || p.FileMIME == "image/png" || p.FileMIME == "image/gif"
	if isImage {
		imgConfig, _, err := image.DecodeConfig(bytes.NewReader(buf))
		if err != nil {
			return fmt.Errorf("unsupported filetype")
		}
		p.FileWidth, p.FileHeight = imgConfig.Width, imgConfig.Height

		return createPostThumbnail(p, buf, p.FileMIME, false, thumbPath)
	}

	ffmpegThumbnail := strings.HasPrefix(p.FileMIME, "image/") || strings.HasPrefix(p.FileMIME, "video/")
	if !ffmpegThumbnail {
		p.Thumb = ""
		return nil
	}

	cmd := exec.Command("ffprobe", "-hide_banner", "-loglevel", "error", "-of", "csv=p=0", "-select_streams", "v", "-show_entries", "stream=width,height", srcPath)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to create thumbnail: %s", err)
	}
	split := bytes.Split(bytes.TrimSpace(out), []byte(","))
	if len(split) >= 2 {
		p.FileWidth, p.FileHeight = ParseInt(string(split[0])), ParseInt(string(split[1]))
	}

	quarterDuration := "0"
	cmd = exec.Command("ffprobe", "-hide_banner", "-loglevel", "error", "-of", "csv=p=0", "-show_entries", "format=duration", srcPath)
	out, err = cmd.Output()
	if err == nil {
		v, err := strconv.ParseFloat(string(bytes.TrimSpace(out)), 64)
		if err == nil {
			quarterDuration = fmt.Sprintf("%f", v/4)
		}
	}

	cmd = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-ss", quarterDuration, "-i", srcPath, "-frames:v", "1", "-vf", fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease", p.Board.ThumbWidth, p.Board.ThumbHeight), thumbPath)
	_, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to create thumbnail: %s", err)
	}

	cmd = exec.Command("ffprobe", "-hide_banner", "-loglevel", "error", "-of", "csv=p=0", "-select_streams", "v", "-show_entries", "stream=width,height", thumbPath)
	out, err = cmd.Output()
	if err == nil {
		split := bytes.Split(bytes.TrimSpace(out), []byte(","))
		if len(split) >= 2 {
			p.ThumbWidth, p.ThumbHeight = ParseInt(string(split[0])), ParseInt(string(split[1]))

			if strings.HasPrefix(p.FileMIME, "video/") {
				thumbData, err := os.ReadFile(thumbPath)
				if err != nil {
					log.Fatal(err)
				}

				err = createPostThumbnail(p, thumbData, "image/jpeg", true, thumbPath)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}
	return nil
}

func (s *Server) checkDuplicateFileHash(db *database.DB, post *Post) *Post {
	if post.FileHash == "" || post.Board.Instances == 0 {
		return nil
	}
	var allowed int
	var filterBoard *Board
	if post.Board.Instances > 0 {
		allowed = post.Board.Instances
	} else {
		allowed = -post.Board.Instances
		filterBoard = post.Board
	}
	matches := db.PostsByFileHash(post.FileHash, filterBoard)
	if len(matches) >= allowed {
		return matches[0]
	}
	return nil
}

func (s *Server) servePost(db *database.DB, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid request", http.StatusInternalServerError)
		return
	}

	boardDir := FormString(r, "board")
	b := db.BoardByDir(boardDir)
	if b == nil {
		data := s.buildData(db, w, r)
		data.BoardError(w, Get(b, data.Account, "No board specified."))
		return
	}

	var (
		rawHTML      bool
		staffPost    bool
		staffCapcode string
	)
	data := s.buildData(db, w, r)
	if data.Account != nil {
		staffPost = FormString(r, "capcode") != ""
		if staffPost {
			capcode := FormInt(r, "capcode")
			if capcode < 0 || capcode > 2 || (data.Account.Role == RoleMod && capcode == 2) {
				capcode = 0
			}
			switch capcode {
			case 1:
				staffCapcode = "Mod"
			case 2:
				staffCapcode = "Admin"
			}

			rawHTML = FormBool(r, "raw")
		}
	}

	switch b.Lock {
	case LockPost:
		if !staffPost {
			data := s.buildData(db, w, r)
			data.BoardError(w, Get(b, data.Account, "Board locked. No new posts may be created."))
			return
		}
	case LockStaff:
		data := s.buildData(db, w, r)
		data.BoardError(w, Get(b, data.Account, "Board locked. No new posts may be created."))
		return
	}

	now := time.Now().Unix()
	post := &Post{
		Board:     b,
		Timestamp: now,
		Bumped:    now,
		Moderated: 1,
	}

	post.IP = s.hashIP(r)

	if b.Delay != 0 {
		lastPost := db.LastPostByIP(post.Board, post.IP)
		if lastPost != nil {
			nextPost := lastPost.Timestamp + int64(b.Delay)
			if time.Now().Unix() < nextPost {
				waitTime := time.Until(time.Unix(nextPost, 0)) // This should be rounded to the nearest second. Oh well.
				data := s.buildData(db, w, r)
				data.BoardError(w, Get(b, data.Account, "Please wait %s before creating a new post.", waitTime))
				return
			}
		}
	}

	err := s.loadPostForm(db, r, post)
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
			data.BoardError(w, Get(b, data.Account, "No post selected."))
			return
		}
	}

	oekakiPost := b.Oekaki && FormBool(r, "oekaki")

	var solvedCAPTCHA *CAPTCHA
	if !staffPost {
		if b.Lock == LockThread && parentPost == nil {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, Get(b, data.Account, "You may only reply to threads."))
			return
		}
		if s.opt.CAPTCHA {
			expired := db.ExpiredCAPTCHAs()
			for _, c := range expired {
				db.DeleteCAPTCHA(c.IP)
				os.Remove(filepath.Join(s.config.Root, "captcha", c.Image+".png"))
			}

			challenge := db.GetCAPTCHA(post.IP)
			if challenge != nil {
				solution := FormString(r, "captcha")
				if strings.ToLower(solution) == challenge.Text {
					solvedCAPTCHA = challenge
				}
			}
			if solvedCAPTCHA == nil {
				s.deletePostFiles(post)

				data := s.buildData(db, w, r)
				data.BoardError(w, Get(b, data.Account, "Incorrect CAPTCHA text. Please try again."))
				return
			}
		}
	}

	files, err := s.loadPostFiles(r, post)
	if err != nil {
		data := s.buildData(db, w, r)
		data.BoardError(w, err.Error())
		return
	}

	if oekakiPost && len(files) == 0 {
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
	if solvedCAPTCHA != nil {
		db.DeleteCAPTCHA(post.IP)
		os.Remove(filepath.Join(s.config.Root, "captcha", solvedCAPTCHA.Image+".png"))
	}

	if post.File == "" && len(b.Embeds) > 0 {
		embed := FormString(r, "embed")
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

				fileExt := MIMEToExt(mimeType)
				if fileExt == "" {
					continue
				}

				thumbName := fmt.Sprintf("%d.%s", time.Now().UnixNano(), fileExt)
				thumbPath := filepath.Join(s.config.Root, b.Dir, "thumb", thumbName)

				err = createPostThumbnail(post, buf, mimeType, true, thumbPath)
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
				data.BoardError(w, Get(b, data.Account, "Failed to embed media."))
				return
			}
		}
	}

	var remainingFiles []*multipart.FileHeader
	if post.File == "" && len(files) > 0 {
		err = s.loadPostFile(db, r, post, files[0])
		if err != nil {
			data := s.buildData(db, w, r)
			data.BoardError(w, err.Error())
			return
		} else if len(files) > 1 {
			remainingFiles = files[1:]
		}

	}

	duplicate := s.checkDuplicateFileHash(db, post)
	if duplicate != nil {
		var postLink string
		if duplicate.Moderated != ModeratedHidden {
			postLink = fmt.Sprintf(` <a href="%sres/%d.html#%d">here</a>`, duplicate.Board.Path(), duplicate.Thread(), duplicate.ID)
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

	if rawHTML {
		post.Message = html.UnescapeString(post.Message)
	}

	var addReport bool
	if !staffPost {
		if parentPost != nil && parentPost.Locked {
			data := s.buildData(db, w, r)
			data.BoardError(w, Get(b, data.Account, "That thread is locked."))
			return
		}

		for _, keyword := range db.AllKeywords() {
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
					existing := db.BanByIP(post.IP)
					if existing == nil {
						ban := &Ban{
							IP:        post.IP,
							Timestamp: time.Now().Unix(),
							Expire:    banExpire,
							Reason:    Get(b, data.Account, "Detected banned keyword."),
						}
						db.AddBan(ban)

						s.log(db, nil, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info()+fmt.Sprintf(" Detected >>/keyword/%d", keyword.ID))
					}
				}

				if action == "delete" || action == "ban" {
					s.deletePostFiles(post)

					data := s.buildData(db, w, r)
					data.BoardError(w, Get(b, data.Account, "Detected banned keyword."))
					return
				}
			}
		}

		if post.FileHash != "" && db.FileBanned(post.FileHash) {
			ban := &Ban{
				IP:        post.IP,
				Timestamp: time.Now().Unix(),
				Reason:    Get(b, data.Account, "Detected banned file."),
			}
			db.AddBan(ban)

			s.log(db, nil, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info())
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, Get(b, data.Account, "Detected banned file."))
			return
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
			db.Plugin = info.Name
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
		db.Plugin = ""

		var foundURL bool
		post.Message = URLPattern.ReplaceAllStringFunc(post.Message, func(s string) string {
			foundURL = true
			match := URLPattern.FindStringSubmatch(s)
			return fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, match[1], match[1])
		})
		if foundURL {
			post.Message = FixURLPattern1.ReplaceAllString(post.Message, `(<a href="$1" target="_blank">$2</a>)`)
			post.Message = FixURLPattern2.ReplaceAllString(post.Message, `<a href="$1" target="_blank">$2</a>.`)
			post.Message = FixURLPattern3.ReplaceAllString(post.Message, `<a href="$1" target="_blank">$2</a>,`)
		}

		post.Message = RefLinkPattern.ReplaceAllStringFunc(post.Message, func(s string) string {
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

		var allBoards []*Board
		post.Message = BoardLinkPattern.ReplaceAllStringFunc(post.Message, func(s string) string {
			if allBoards == nil {
				allBoards = db.AllBoards()
			}
			path := strings.TrimSuffix(strings.TrimPrefix(s[12:], "/"), "/")
			for _, b := range allBoards {
				if b.Dir == path {
					return fmt.Sprintf(`<a href="%s">&gt;&gt;&gt;%s</a>`, b.Path(), b.Path())
				}
			}
			return s
		})

		var quote bool
		lines := strings.Split(post.Message, "\n")
		for i := range lines {
			lines[i] = QuotePattern.ReplaceAllStringFunc(lines[i], func(s string) string {
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

	post.SetNameBlock(b.DefaultName, staffCapcode, s.opt.Identifiers)

	if !rawHTML {
		newLineSentinel := "\x85" // Next line (NEL) character
		post.Message = strings.ReplaceAll(post.Message, "\n", "<br>\n")
		post.Message = strings.ReplaceAll(post.Message, newLineSentinel, "\n")
		bracketSentinel := "\x1e" // Record separator
		post.Message = strings.ReplaceAll(post.Message, bracketSentinel, "[")
	}

	if post.Password != "" {
		post.Password = s.hashData(post.Password)
	}

	if !staffPost && (b.Approval == ApprovalAll || (b.Approval == ApprovalFile && post.File != "")) {
		post.Moderated = 0
	}

	postCopy := post.Copy()
	for _, info := range allPluginInsertHandlers {
		db.Plugin = info.Name
		err := info.Handler(db, postCopy)
		if err != nil {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, err.Error())
			return
		}
	}
	db.Plugin = ""

	db.AddPost(post)

	posts := []*Post{post}
	cancel := func() {
		for _, p := range posts {
			s.deletePostFiles(p)
		}
		db.SoftRollBack()
	}
	for _, fileHeader := range remainingFiles {
		p := post.Copy()
		p.ID = 0
		if post.Parent == 0 {
			p.Parent = post.ID
		}
		p.Subject = ""
		p.Message = ""
		p.ResetAttachment()

		err = s.loadPostFile(db, r, p, fileHeader)
		if err != nil {
			cancel()

			data := s.buildData(db, w, r)
			data.BoardError(w, err.Error())
			return
		}

		duplicate := s.checkDuplicateFileHash(db, p)
		if duplicate != nil {
			cancel()

			var postLink string
			if duplicate.Moderated != ModeratedHidden {
				postLink = fmt.Sprintf(` <a href="%sres/%d.html#%d">here</a>`, duplicate.Board.Path(), duplicate.Thread(), duplicate.ID)
			}

			var uploadType = "file"
			if p.IsEmbed() {
				uploadType = "embed"
			}

			data := s.buildData(db, w, r)
			data.Template = "board_error"
			data.Info = fmt.Sprintf("Duplicate %s uploaded.", uploadType)
			data.Message = template.HTML(fmt.Sprintf(`<div style="text-align: center;">That %s has already been posted%s.</div><br>`, uploadType, postLink))
			data.execute(w)
			return
		}

		db.AddPost(p)
		posts = append(posts, p)
	}

	postCopy = post.Copy()
	for _, info := range allPluginCreateHandlers {
		db.Plugin = info.Name
		err := info.Handler(db, postCopy)
		if err != nil {
			cancel()

			log.Fatalf("plugin %s failed to process create event: %s", info.Name, err)
		}
	}
	db.Plugin = ""

	if post.Moderated == ModeratedHidden {
		data.Template = "board_info"
		data.Info = Get(b, data.Account, "Your post will be shown once it has been approved.")
		data.execute(w)
		return
	} else if addReport {
		report := &Report{
			Board:     b,
			Post:      post,
			Timestamp: time.Now().Unix(),
			IP:        s.hashIP(r),
		}
		db.AddReport(report)
	}

	if post.Parent == 0 {
		for _, thread := range db.TrimThreads(post.Board) {
			s.deletePost(db, thread)
		}
	} else if strings.ToLower(post.Email) != "sage" {
		bump := post.Board.MaxReplies == 0 || db.ReplyCount(post.Parent) <= post.Board.MaxReplies
		if bump {
			db.BumpThread(post.Parent, now)
		}
	}

	s.rebuildLock.Lock()
	db.Commit()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	s.lock.Unlock()
	s.rebuildQueue <- &rebuildInfo{post: post, wg: wg}
	s.rebuildLock.Unlock()

	wg.Wait()

	redir := fmt.Sprintf("%sres/%d.html#%d", b.Path(), post.Thread(), post.ID)
	http.Redirect(w, r, redir, http.StatusFound)

	s.lock.Lock()
}
