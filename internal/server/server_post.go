package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

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
			return nil, errors.New(Get(b, nil, "Unsupported file format."))
		}
	case "image/gif":
		img, err = gif.Decode(r)
		if err != nil {
			return nil, errors.New(Get(b, nil, "Unsupported file format."))
		}
	case "image/png":
		img, err = png.Decode(r)
		if err != nil {
			return nil, errors.New(Get(b, nil, "Unsupported file format."))
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
			return errors.New(Get(nil, nil, "Unsupported file format."))
		}
	case "image/gif":
		err = gif.Encode(file, img, nil)
		if err != nil {
			return errors.New(Get(nil, nil, "Unsupported file format."))
		}
	case "image/png":
		err = png.Encode(file, img)
		if err != nil {
			return errors.New(Get(nil, nil, "Unsupported file format."))
		}
	}
	return nil
}

func createPostThumbnail(p *Post, file io.Reader, mimeType string, mediaOverlay bool, thumbPath string) error {
	thumbImg, err := resizeImage(p.Board, file, mimeType)
	if err != nil {
		return errors.New(Get(p.Board, nil, "Unsupported file format."))
	}

	if mediaOverlay {
		thumbImg = p.AddMediaOverlay(thumbImg)
	}

	bounds := thumbImg.Bounds()
	p.ThumbWidth, p.ThumbHeight = bounds.Dx(), bounds.Dy()

	err = writeImage(thumbImg, mimeType, thumbPath)
	if err != nil {
		return errors.New(Get(p.Board, nil, "Unsupported file format."))
	}
	return nil
}

func setFileAndThumb(p *Post, rootDir string, fileExt string, thumbExt string) {
	postUploadFileLock.Lock()
	defer postUploadFileLock.Unlock()

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

	fileID := time.Now().UnixMilli()
	for {
		fileIDString := fmt.Sprintf("%d", fileID)
		fileName := fileIDString + "." + fileExt
		thumbName := fileIDString + "s." + thumbExt

		// Check whether file already exists.
		_, err := os.Stat(filepath.Join(rootDir, p.Board.Dir, "src", fileName))
		if err == nil {
			fileID++
			continue
		} else if !os.IsNotExist(err) {
			log.Fatal(err)
		}

		// Check whether thumbnail already exists.
		_, err = os.Stat(filepath.Join(rootDir, p.Board.Dir, "thumb", thumbName))
		if err == nil {
			fileID++
			continue
		} else if !os.IsNotExist(err) {
			log.Fatal(err)
		}

		p.File = fileName
		p.Thumb = thumbName
		return
	}
}

func (s *Server) loadPostForm(db serverDB, r *http.Request, p *Post) error {
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
			return errors.New(Get(p.Board, nil, "Please enter a name."))
		}
		return errors.New(Get(p.Board, nil, "Please enter a name at least %d characters long.", p.Board.MinName))
	}
	if len(p.Email) < p.Board.MinEmail {
		if p.Board.MinEmail == 1 {
			return errors.New(Get(p.Board, nil, "Please enter an email address."))
		}
		return errors.New(Get(p.Board, nil, "Please enter an email address at least %d characters long.", p.Board.MinEmail))
	}
	if len(p.Subject) < p.Board.MinSubject && (p.Board.Type == TypeImageboard || p.Parent == 0) {
		if p.Board.MinSubject == 1 {
			return errors.New(Get(p.Board, nil, "Please enter a subject."))
		}
		return errors.New(Get(p.Board, nil, "Please enter a subject at least %d characters long.", p.Board.MinSubject))
	}
	if len(p.Message) < p.Board.MinMessage {
		if p.Board.MinMessage == 1 {
			return errors.New(Get(p.Board, nil, "Please enter a message."))
		}
		return errors.New(Get(p.Board, nil, "Please enter a message at least %d characters long.", p.Board.MinMessage))
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
			return nil, errors.New(Get(p.Board, nil, "File uploads are not allowed."))
		}
		return nil, errors.New(GetN(p.Board, nil, "Only %d file may be uploaded at once.", "Only %d files may be uploaded at once.", p.Board.Files))
	}
	return files, nil
}

func (s *Server) loadPostFile(db serverDB, r *http.Request, p *Post, fileHeader *multipart.FileHeader) error {
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
				return errors.New(Get(p.Board, nil, "A file is required."))
			}
			return errors.New(Get(p.Board, nil, "An attachment is required."))
		}
		return errors.New(Get(p.Board, nil, "A file %s or larger is required.", FormatFileSize(minSize)))
	} else if fileHeader.Size > maxSize {
		return errors.New(Get(p.Board, nil, "Maximum file size allowed is %s.", FormatFileSize(maxSize)))
	}

	formFile, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer formFile.Close()

	mime, err := mimetype.DetectReader(formFile)
	if err == nil {
		p.FileMIME = mime.String()
	}
	p.FileOriginal = fileHeader.Filename

	oekakiPost := p.Board.Oekaki && p.FileMIME == "application/octet-stream"
	if oekakiPost {
		buf := make([]byte, 3)
		formFile.Seek(0, 0)
		formFile.Read(buf)
		oekakiPost = buf[0] == 0x54 && buf[1] == 0x47 && buf[2] == 0x4B
		if oekakiPost {
			p.FileMIME = "application/x-tegaki"
		}
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
				db.SetPlugin(info.Name)
				formFile.Seek(0, 0)
				handled, err := info.Handler(db, p, formFile)
				if err != nil {
					db.SetPlugin("")
					return err
				} else if handled {
					db.SetPlugin("")
					return nil
				}
			}
			db.SetPlugin("")

			var extra string
			if s.opt.DevMode && p.FileMIME != "" {
				extra = " (" + p.FileMIME + ")"
			}
			return errors.New(Get(p.Board, nil, "Unsupported file format.") + extra)
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

	setFileAndThumb(p, s.config.Root, fileExt, thumbExt)

	p.FileSize = fileHeader.Size
	if oekakiPost && FormBool(r, "oekaki") {
		p.FileOriginal = FormString(r, "title")
	}

	srcPath := filepath.Join(s.config.Root, p.Board.Dir, "src", p.File)
	thumbPath := filepath.Join(s.config.Root, p.Board.Dir, "thumb", p.Thumb)

	file, err := os.OpenFile(srcPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	err = preallocateFile(file, fileHeader.Size)
	if err != nil {
		log.Fatal(err)
	}

	formFile.Seek(0, 0)

	hash := s.newHash()
	tee := io.TeeReader(formFile, hash)

	wrote, err := io.Copy(file, tee)
	if err != nil {
		log.Fatal(err)
	} else if wrote != fileHeader.Size {
		log.Fatalf("failed to store uploaded file: tried to write %d bytes, but only %d bytes were written to disk (is the disk full?)", fileHeader.Size, wrote)
	}

	file.Close()

	var sum [HashSize]byte
	hash.Sum(sum[:0])
	p.FileHash = base64.URLEncoding.EncodeToString(sum[:])

	if oekakiPost {
		formThumb, formThumbHeader, err := r.FormFile("thumb")
		if err != nil || formThumbHeader == nil || formThumbHeader.Size < minSize {
			return fmt.Errorf("a thumbnail is required")
		}

		buf, err := io.ReadAll(formThumb)
		if err != nil {
			log.Fatal(err)
		}

		imgWidth, imgHeight := s.imageDimensions(bytes.NewReader(buf))
		if imgWidth == 0 || imgHeight == 0 {
			return fmt.Errorf("unsupported thumbnail filetype")
		}
		p.FileWidth, p.FileHeight = imgWidth, imgHeight

		return createPostThumbnail(p, bytes.NewReader(buf), "image/png", false, thumbPath)
	}

	if fileThumb == "none" {
		p.Thumb = ""
		return nil
	} else if fileThumb != "" {
		return createPostThumbnail(p, bytes.NewReader(thumbData), mimetype.Detect(thumbData).String(), false, thumbPath)
	}

	isImage := p.FileMIME == "image/jpeg" || p.FileMIME == "image/pjpeg" || p.FileMIME == "image/png" || p.FileMIME == "image/gif"
	if isImage {
		formFile.Seek(0, 0)
		imgWidth, imgHeight := s.imageDimensions(formFile)
		if imgWidth == 0 || imgHeight == 0 {
			return errors.New(Get(p.Board, nil, "Unsupported file format."))
		}
		p.FileWidth, p.FileHeight = imgWidth, imgHeight

		formFile.Seek(0, 0)
		return createPostThumbnail(p, formFile, p.FileMIME, false, thumbPath)
	}

	ffmpegThumbnail := strings.HasPrefix(p.FileMIME, "image/") || strings.HasPrefix(p.FileMIME, "video/")
	if !ffmpegThumbnail {
		p.Thumb = ""
		return nil
	}

	cmd := exec.Command("ffprobe", "-hide_banner", "-loglevel", "error", "-of", "csv=p=0", "-select_streams", "v", "-show_entries", "stream=width,height", srcPath)
	out, err := cmd.Output()
	if err != nil {
		return errors.New(Get(p.Board, nil, "Failed to create thumbnail: %s", err))
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
		return errors.New(Get(p.Board, nil, "Failed to create thumbnail: %s", err))
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

				err = createPostThumbnail(p, bytes.NewReader(thumbData), "image/jpeg", true, thumbPath)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}
	return nil
}

func (s *Server) checkDuplicateFileHash(db serverDB, post *Post) *Post {
	if post.FileHash == "" || post.Board.Instances == 0 {
		return nil
	}
	var filterBoard *Board
	allowed := post.Board.Instances
	if allowed < 0 {
		allowed *= -1
		filterBoard = post.Board
	}
	matches := db.PostsByFileHash(post.FileHash, filterBoard)
	if len(matches) >= allowed {
		return matches[0]
	}
	return nil
}

func (s *Server) embedMedia(db serverDB, post *Post, embed string, dryRun bool) error {
	b := post.Board
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

		requestURL := strings.ReplaceAll(embedURL, "SRIRACHA_EMBED", embed)
		req, err := http.NewRequest(http.MethodGet, requestURL, nil)
		if err != nil {
			continue
		}

		resp, err := s.httpResponse(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		info := &embedInfo{}
		err = json.NewDecoder(resp.Body).Decode(&info)
		if err != nil || info.Title == "" || info.Thumb == "" || info.HTML == "" || !strings.HasPrefix(info.Thumb, "https://") {
			continue
		}

		// YouTube returns a low quality 4:3 thumbnail by default.
		// Replace with high quality 16:9 thumbnail when available.
		var backupThumb string
		u, err := url.Parse(embed)
		if err == nil {
			var ytVideoID string
			switch strings.ToLower(u.Host) {
			case "youtube.com", "www.youtube.com":
				ytVideoID = u.Query().Get("v")
			case "youtu.be", "www.youtu.be":
				ytVideoID = strings.TrimPrefix(u.Path, "/")
			}
			if ytVideoID != "" && AlphaNumericAndSymbols.MatchString(ytVideoID) {
				backupThumb = info.Thumb
				info.Thumb = "https://img.youtube.com/vi/" + ytVideoID + "/maxresdefault.jpg"
			}
		}

		// Fetch thumbnail.
		thumbReq, err := http.NewRequest(http.MethodGet, info.Thumb, nil)
		if err != nil {
			continue
		}
		thumbResp, err := s.httpResponse(thumbReq)
		respOK := thumbResp != nil && thumbResp.StatusCode >= 200 && thumbResp.StatusCode < 300
		if err != nil || !respOK {
			if !respOK {
				thumbResp.Body.Close()
			}
			if backupThumb == "" {
				continue
			}

			// Retry using backup thumbnail URL.
			thumbReq, err = http.NewRequest(http.MethodGet, backupThumb, nil)
			if err != nil {
				continue
			}
			thumbResp, err = s.httpResponse(thumbReq)
			if err != nil {
				continue
			}
		}
		buf, err := io.ReadAll(thumbResp.Body)
		thumbResp.Body.Close()
		if err != nil {
			continue
		}

		post.File = info.HTML
		post.FileHash = "e " + embedName + " " + info.Title
		post.FileOriginal = embed
		if dryRun {
			break
		}

		mimeType := mimetype.Detect(buf).String()

		fileExt := MIMEToExt(mimeType)
		if fileExt == "" {
			continue
		}

		post.Thumb = fmt.Sprintf("%d.%s", time.Now().UnixNano(), fileExt)
		thumbPath := filepath.Join(s.config.Root, b.Dir, "thumb", post.Thumb)

		err = createPostThumbnail(post, bytes.NewReader(buf), mimeType, true, thumbPath)
		if err != nil {
			continue
		}
		break
	}

	if post.File == "" {
		for _, info := range allPluginEmbedHandlers {
			db.SetPlugin(info.Name)
			handled, err := info.Handler(db, post, embed)
			if err != nil {
				db.SetPlugin("")
				return err
			} else if handled {
				break
			}
		}
		db.SetPlugin("")

		if post.File == "" {
			return fmt.Errorf(Get(b, nil, "Failed to embed media."))
		}
	}
	return nil
}

func (s *Server) servePost(db serverDB, w http.ResponseWriter, r *http.Request) (unlocked bool) {
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
		loggedIn bool
		rawHTML  bool
		capcode  string
	)
	data := s.buildData(db, w, r)
	if data.Account != nil {
		loggedIn = true
		if FormString(r, "capcode") != "" {
			rawHTML = FormBool(r, "raw")
			capcodeInt := FormInt(r, "capcode")
			if capcodeInt < 0 || capcodeInt > 2 || (data.Account.Role == RoleMod && capcodeInt == 2) {
				capcodeInt = 0
			}
			switch capcodeInt {
			case 1:
				capcode = "Mod"
			case 2:
				capcode = "Admin"
			}
		}
	}

	switch b.Lock {
	case LockPost:
		if !loggedIn {
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

	var addReport bool
	var solvedCAPTCHA *CAPTCHA
	if !loggedIn {
		if b.Lock == LockThread && parentPost == nil {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, Get(b, data.Account, "Board locked. You may only reply to threads."))
			return
		}

		if s.opt.CAPTCHA {
			expired := db.ExpiredCAPTCHAs()
			if len(expired) != 0 {
				s.captchaCacheLock.Lock()
				for _, c := range expired {
					delete(s.captchaCache, c.IP)
					db.DeleteCAPTCHA(c.IP)
					os.Remove(filepath.Join(s.config.Root, "captcha", c.Image+".png"))
				}
				s.captchaCacheLock.Unlock()
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

		events := []ThresholdEvent{EventPost}
		if post.Parent == 0 {
			events = append(events, EventThread)
		}
		timeout, threshold := s.checkThresholds(db, now, post.IP, events...)
		if timeout != 0 {
			action := s.handleBanAction(db, data.Account, threshold.Action, post.IP, Get(nil, nil, "Exceeded %s threshold.", strings.ToLower(Get(nil, nil, "Post"))), fmt.Sprintf("Exceeded >>/threshold/%d", threshold.ID))
			switch action {
			case "hide":
				post.Moderated = 0
			case "report":
				addReport = true
			case "delete":
				s.deletePostFiles(post)

				var typeLabel string
				if post.Parent == 0 {
					typeLabel = G(b, data.Account, "Thread")
				} else {
					typeLabel = G(b, data.Account, "Post")
				}
				data := s.buildData(db, w, r)
				data.BoardError(w, Get(b, data.Account, "Please wait %[1]s before creating a new %[2]s.", FormatDuration(time.Duration(timeout)*time.Second), strings.ToLower(typeLabel)))
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
		s.captchaCacheLock.Lock()
		delete(s.captchaCache, post.IP)
		s.captchaCacheLock.Unlock()
		db.DeleteCAPTCHA(post.IP)
		os.Remove(filepath.Join(s.config.Root, "captcha", solvedCAPTCHA.Image+".png"))
	}

	if post.File == "" && len(b.Embeds) > 0 {
		embed := FormString(r, "embed")
		if embed != "" {
			err = s.embedMedia(db, post, embed, false)
			if err != nil {
				data := s.buildData(db, w, r)
				data.BoardError(w, err.Error())
				return
			}
		}
	}

	var remainingFiles []*multipart.FileHeader
	if post.File == "" && len(files) > 0 {
		err = s.loadPostFile(db, r, post, files[0])
		if err != nil {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, err.Error())
			return
		} else if len(files) > 1 {
			remainingFiles = files[1:]
		}
	}

	if post.Thumb != "" && FormBool(r, "spoiler") {
		post.FileOriginal = "!" + post.FileOriginal
	}

	duplicate := s.checkDuplicateFileHash(db, post)
	if duplicate != nil {
		s.deletePostFiles(post)

		var postLink template.HTML
		if duplicate.Moderated != ModeratedHidden {
			postLink = "<br>" + duplicate.RefLink()
		}

		var info string
		var msg string
		if post.IsEmbed() {
			info = "Duplicate embed detected."
			msg = "That embed has already been posted."
		} else {
			info = "Duplicate file detected."
			msg = "That file has already been posted."
		}

		data := s.buildData(db, w, r)
		data.Template = "board_error"
		data.Info = G(post.Board, nil, info)
		data.Message = template.HTML(fmt.Sprintf(`<div style="text-align: center;">%s%s</div><br>`, G(post.Board, nil, msg), postLink))
		data.execute(w)
		return
	}

	if (b.Require == RequireAll || (b.Require == RequireThreads && post.Parent == 0) || (b.Require == RequireReplies && post.Parent != 0)) && post.File == "" {
		data := s.buildData(db, w, r)
		data.BoardError(w, Get(b, data.Account, "An attachment is required."))
		return
	}

	if rawHTML {
		post.Message = html.UnescapeString(post.Message)
	}

	if !loggedIn {
		if parentPost != nil && parentPost.Locked {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, Get(b, data.Account, "That thread is locked."))
			return
		}

		for _, k := range s.keywordCache[post.Board.ID] {
			if !k.p.MatchString(post.Name) && !k.p.MatchString(post.Email) && !k.p.MatchString(post.Subject) && !k.p.MatchString(post.Message) {
				continue
			}

			// Keyword matched. Handle action.
			data := s.buildData(db, w, r)
			action := s.handleBanAction(db, data.Account, k.a, post.IP, Get(nil, nil, "Detected banned keyword."), fmt.Sprintf("Detected >>/keyword/%d", k.id))
			switch action {
			case "hide":
				post.Moderated = 0
			case "report":
				addReport = true
			case "delete":
				s.deletePostFiles(post)

				data := s.buildData(db, w, r)
				data.BoardError(w, Get(b, data.Account, "Detected banned keyword."))
				return
			}
		}

		if post.FileHash != "" && db.FileBanned(post.FileHash) {
			ban := &Ban{
				IP:        post.IP,
				Timestamp: time.Now().Unix(),
				Reason:    Get(nil, nil, "Detected banned file."),
			}
			db.AddBan(ban)

			s.log(db, nil, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info()+" File hash: "+post.FileHash)
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
			db.SetPlugin(info.Name)
			err := info.Handler(db, post)
			if err != nil {
				s.deletePostFiles(post)

				data := s.buildData(db, w, r)
				data.BoardError(w, err.Error())
				return
			}
			post.Message = strings.ReplaceAll(post.Message, "<br>", "\n")
			post.Message = strings.ReplaceAll(post.Message, "<br/>", "\n")
		}
		db.SetPlugin("")

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
		fileOK := maxSize != 0
		embedOK := len(post.Board.Embeds) != 0
		msgOK := post.Board.MaxMessage != 0
		var msg string
		switch {
		case fileOK && embedOK && msgOK:
			msg = "Please upload a file, enter an embed URL or enter a message."
		case fileOK && embedOK:
			msg = "Please upload a file or enter an embed URL."
		case fileOK && msgOK:
			msg = "Please upload a file or enter a message."
		case fileOK:
			msg = "Please upload a file."
		case embedOK && msgOK:
			msg = "Please enter an embed URL or enter a message."
		case embedOK:
			msg = "Please enter an embed URL."
		case msgOK:
			msg = "Please enter a message."
		default:
			msg = "Board locked. No new posts may be created."
		}
		data := s.buildData(db, w, r)
		data.BoardError(w, G(post.Board, nil, msg))
		return
	}

	post.SetNameBlock(b.DefaultName, capcode, s.opt.Identifiers)

	// Replace sentinels with characters.
	if !rawHTML {
		newLineSentinel := "\x85" // Next line (NEL) character
		post.Message = strings.ReplaceAll(post.Message, "\n", "<br>\n")
		post.Message = strings.ReplaceAll(post.Message, newLineSentinel, "\n")
		bracketSentinel := "\x1e" // Record separator
		post.Message = strings.ReplaceAll(post.Message, bracketSentinel, "[")
	}

	// Replace XHTML line break tags added by plugins or by staff posting raw HTML.
	post.Message = strings.ReplaceAll(post.Message, "<br/>", "<br>")

	if post.Password != "" {
		post.Password = s.hashData(post.Password)
	}

	if !loggedIn && (b.Approval == ApprovalAll || (b.Approval == ApprovalFile && post.File != "")) {
		post.Moderated = 0
	}

	postCopy := post.Copy()
	for _, info := range allPluginInsertHandlers {
		db.SetPlugin(info.Name)
		err := info.Handler(db, postCopy)
		if err != nil {
			s.deletePostFiles(post)

			data := s.buildData(db, w, r)
			data.BoardError(w, err.Error())
			return
		}
	}
	db.SetPlugin("")

	preview := FormBool(r, "preview") && !oekakiPost
	if !preview {
		db.AddPost(post)
	}

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

		if p.Thumb != "" && FormBool(r, "spoiler") {
			p.FileOriginal = "!" + p.FileOriginal
		}

		duplicate := s.checkDuplicateFileHash(db, p)
		if duplicate != nil {
			cancel()

			var postLink template.HTML
			if duplicate.Moderated != ModeratedHidden {
				postLink = "<br>" + duplicate.RefLink()
			}

			var info string
			var msg string
			if p.IsEmbed() {
				info = "Duplicate embed detected."
				msg = "That embed has already been posted."
			} else {
				info = "Duplicate file detected."
				msg = "That file has already been posted."
			}

			data := s.buildData(db, w, r)
			data.Template = "board_error"
			data.Info = G(p.Board, nil, info)
			data.Message = template.HTML(fmt.Sprintf(`<div style="text-align: center;">%s%s</div><br>`, G(p.Board, nil, msg), postLink))
			data.execute(w)
			return
		}

		if db.FileBanned(p.FileHash) {
			cancel()

			ban := &Ban{
				IP:        p.IP,
				Timestamp: time.Now().Unix(),
				Reason:    Get(nil, nil, "Detected banned file."),
			}
			db.AddBan(ban)

			s.log(db, nil, nil, fmt.Sprintf("Added >>/ban/%d", ban.ID), ban.Info()+" File hash: "+p.FileHash)
			s.deletePostFiles(p)

			data := s.buildData(db, w, r)
			data.BoardError(w, Get(b, data.Account, "Detected banned file."))
			return
		}

		if !preview {
			db.AddPost(p)
		}
		posts = append(posts, p)
	}

	if preview {
		lastID := db.MaxPostID()
		previewThread := lastID + 1
		for i, p := range posts {
			lastID++
			p.ID = lastID
			if i == 0 {
				continue
			}
			p.Parent = previewThread
		}
		data.Template = "board_page"
		data.Board = post.Board
		data.Threads = [][]*Post{posts}
		data.Preview = true
		data.ReplyMode = previewThread
		for _, post := range posts {
			if post.Thumb != "" {
				data.Extra = "thumb"
				break
			}
		}
		data.execute(w)

		cancel()
		return
	}

	postCopy = post.Copy()
	for _, info := range allPluginCreateHandlers {
		db.SetPlugin(info.Name)
		err := info.Handler(db, postCopy)
		if err != nil {
			cancel()

			log.Fatalf("plugin %s failed to process create event: %s", info.Name, err)
		}
	}
	db.SetPlugin("")

	if post.Moderated == ModeratedHidden {
		data.Template = "board_info"
		data.Info = Get(b, data.Account, "Your post will be shown once it has been approved.")
		data.execute(w)
		s.writeModQueue(db)
		return
	} else if addReport {
		report := &Report{
			Board:     b,
			Post:      post,
			Timestamp: time.Now().Unix(),
			IP:        s.hashIP(r),
		}
		db.AddReport(report)
		s.writeModQueue(db)
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

	if post.Moderated != ModeratedHidden {
		db.AddPostBacklinks(post)
	}

	s.rebuildLock.Lock()
	db.Commit()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	s.lock.Unlock()
	unlocked = true

	s.rebuildQueue <- &rebuildInfo{post: post, wg: wg}
	s.rebuildLock.Unlock()
	wg.Wait()

	redir := fmt.Sprintf("%sres/%d.html#%d", b.Path(), post.Thread(), post.ID)
	data.Redirect(w, r, redir)
	return
}
