package sriracha

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/nfnt/resize"
)

type PostModerated int

const (
	ModeratedHidden   PostModerated = 0
	ModeratedVisible  PostModerated = 1
	ModeratedApproved PostModerated = 2
)

type Post struct {
	ID           int
	Board        *Board
	Parent       int
	Timestamp    int64
	Bumped       int64
	IP           string
	Name         string
	Tripcode     string
	Email        string
	NameBlock    string
	Subject      string
	Message      string
	Password     string
	File         string
	FileHash     string
	FileOriginal string
	FileSize     int64
	FileWidth    int
	FileHeight   int
	Thumb        string
	ThumbWidth   int
	ThumbHeight  int
	Moderated    PostModerated
	Stickied     int
	Locked       int
}

var postUploadFileLock = &sync.Mutex{}

func (p *Post) setFileAndThumb(fileExt string, thumbExt string) {
	postUploadFileLock.Lock()
	defer postUploadFileLock.Unlock()

	fileID := time.Now().UnixNano()
	fileIDString := fmt.Sprintf("%d", fileID)

	if thumbExt != "" {
		thumbExt = fileExt
	} else if fileExt != "jpg" && fileExt != "png" && fileExt != "gif" {
		thumbExt = "jpg"
	}

	p.File = fileIDString + "." + fileExt
	p.Thumb = fileIDString + "s." + thumbExt
}

func (p *Post) setFileAttributes(buf []byte, name string) error {
	checksum := sha512.Sum384(buf)
	p.FileHash = base64.URLEncoding.EncodeToString(checksum[:])

	p.FileOriginal = name

	p.FileSize = int64(len(buf))
	return nil
}

func (p *Post) createThumbnail(buf []byte, mimeType string, thumbPath string) error {
	thumbImg, err := resizeImage(p.Board, bytes.NewReader(buf), mimeType)
	if err != nil {
		return err
	}

	bounds := thumbImg.Bounds()
	p.ThumbWidth, p.ThumbHeight = bounds.Dx(), bounds.Dy()

	err = writeImage(thumbImg, mimeType, thumbPath)
	if err != nil {
		return fmt.Errorf("unsupported filetype")
	}
	return nil
}

func (p *Post) loadForm(r *http.Request, rootDir string) error {
	p.Parent = formInt(r, "parent")

	p.Name = formString(r, "name")
	p.Email = formString(r, "email")
	p.Subject = formString(r, "subject")
	p.Message = html.EscapeString(formString(r, "message"))

	formFile, formFileHeader, err := r.FormFile("file")
	if err != nil || formFileHeader == nil {
		return nil
	}

	buf, err := io.ReadAll(formFile)
	if err != nil {
		log.Fatal(err)
	}

	mimeType := mimetype.Detect(buf).String()

	var fileExt string
	var fileThumb string
	if p.Board.HasUpload(mimeType) {
		for _, u := range srirachaServer.config.UploadTypes() {
			if u.MIME == mimeType {
				fileExt = u.Ext
				fileThumb = u.Thumb
				break
			}
		}
	}
	if fileExt == "" {
		log.Println(mimeType, "!")
		return fmt.Errorf("unsupported filetype")
	}

	var thumbExt string
	var thumbData []byte
	if fileThumb != "" && fileThumb != "none" {
		thumbData, err = templateFS.ReadFile("template/img/" + fileThumb)
		if err != nil {
			log.Fatalf("failed to open thumbnail file %s: %s", fileThumb, err)
		}

		thumbExt = mimeToExt(mimetype.Detect(thumbData).String())
	}

	p.setFileAndThumb(fileExt, thumbExt)

	err = p.setFileAttributes(buf, formFileHeader.Filename)
	if err != nil {
		return err
	}

	srcPath := filepath.Join(rootDir, p.Board.Dir, "src", p.File)
	thumbPath := filepath.Join(rootDir, p.Board.Dir, "thumb", p.Thumb)

	err = os.WriteFile(srcPath, buf, 0600)
	if err != nil {
		log.Fatal(err)
	}

	if fileThumb == "none" {
		p.Thumb = ""
		return nil
	} else if fileThumb != "" {
		return p.createThumbnail(thumbData, mimetype.Detect(thumbData).String(), thumbPath)
	}

	isImage := mimeType == "image/jpeg" || mimeType == "image/pjpeg" || mimeType == "image/png" || mimeType == "image/gif"
	if isImage {
		imgConfig, _, err := image.DecodeConfig(bytes.NewReader(buf))
		if err != nil {
			return fmt.Errorf("unsupported filetype")
		}
		p.FileWidth, p.FileHeight = imgConfig.Width, imgConfig.Height

		return p.createThumbnail(buf, mimeType, thumbPath)
	}

	isVideo := strings.HasPrefix(mimeType, "video/")
	if !isVideo {
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
		p.FileWidth, p.FileHeight = parseInt(string(split[0])), parseInt(string(split[1]))
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
			p.ThumbWidth, p.ThumbHeight = parseInt(string(split[0])), parseInt(string(split[1]))
		}
	}
	return nil
}

func (p *Post) setNameBlock(defaultName string, capcode string) {
	var out strings.Builder

	emailLink := p.Email != "" && strings.ToLower(p.Email) != "noko"

	name := p.Name
	if name == "" {
		name = defaultName
	}

	if emailLink {
		out.WriteString(`<a href="mailto:"` + html.EscapeString(p.Email) + `">`)
	}
	out.WriteString(`<span class="postername">`)
	out.WriteString(html.EscapeString(name))
	out.WriteString(`</span>`)
	if p.Tripcode != "" {
		out.WriteString(`<span class="postertrip">!` + html.EscapeString(p.Tripcode) + `</span>`)
	}
	if emailLink {
		out.WriteString(`</a>`)
	}

	if capcode != "" {
		spanColor := "red"
		if capcode == "Admin" {
			spanColor = "purple"
		}
		out.WriteString(` <span style="color: ` + spanColor + `;">## ` + capcode + `</span>`)
	}

	out.WriteString(" " + p.TimestampLabel())

	p.NameBlock = out.String()
}

func (p *Post) Thread() int {
	if p.Parent == 0 {
		return p.ID
	}
	return p.Parent
}

func (p *Post) FileSizeLabel() string {
	return formatFileSize(p.FileSize)
}

func (p *Post) TimestampLabel() string {
	return formatTimestamp(p.Timestamp)
}

func (p *Post) IsEmbed() bool {
	return len(p.FileHash) > 2 && p.FileHash[1] == ' ' && p.FileHash[0] == 'e'
}

func (p *Post) EmbedInfo() []string {
	if !p.IsEmbed() {
		return nil
	}
	split := strings.SplitN(p.FileHash, " ", 3)
	if len(split) != 3 {
		return nil
	}
	return split
}

func (p *Post) ExpandHTML() template.HTML {
	if p.File == "" {
		return ""
	} else if p.IsEmbed() {
		return template.HTML(p.File)
	}
	srcPath := fmt.Sprintf("%ssrc/%s", p.Board.Path(), p.File)

	isVideo := strings.HasSuffix(p.File, ".mp4") || strings.HasSuffix(p.File, ".webm")
	if isVideo {
		const expandFormat = `<video width="%d" height="%d" style="position: static; pointer-events: inherit; display: inline; max-width: 85vw; height: auto; max-height: 100%%;" controls autoplay loop><source src="%s"></source></video>`
		return template.HTML(url.PathEscape(fmt.Sprintf(expandFormat, p.FileWidth, p.FileHeight, srcPath)))
	}

	isImage := strings.HasSuffix(p.File, ".jpg") || strings.HasSuffix(p.File, ".png") || strings.HasSuffix(p.File, ".gif")
	if !isImage {
		return ""
	}

	const expandFormat = `<a href="%s" onclick="return expandFile(event, '%d');"><img src="%s" width="%d" style="min-width: %dpx;min-height: %dpx;max-width: 85vw;height: auto;"></a>`
	return template.HTML(url.PathEscape(fmt.Sprintf(expandFormat, srcPath, p.ID, srcPath, p.FileWidth, p.ThumbWidth, p.ThumbHeight)))
}

func (p *Post) RefLink() template.HTML {
	return template.HTML(fmt.Sprintf(`<a href="%sres/%d.html#%d">&gt;&gt;%d</a>`, p.Board.Path(), p.Thread(), p.ID, p.ID))
}

func mimeToExt(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/pjpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/png":
		return "png"
	default:
		return ""
	}
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
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
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
