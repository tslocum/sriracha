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
	"path/filepath"
	"strings"
	"sync"
	"time"

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

func (p *Post) setFileAndThumb(fileExt string) {
	postUploadFileLock.Lock()
	defer postUploadFileLock.Unlock()

	fileID := time.Now().UnixNano()
	fileIDString := fmt.Sprintf("%d", fileID)

	p.File = fileIDString + "." + fileExt
	p.Thumb = fileIDString + "s." + fileExt
}

func (p *Post) setFileAttributes(buf []byte, name string) error {
	checksum := sha512.Sum384(buf)
	p.FileHash = base64.URLEncoding.EncodeToString(checksum[:])

	p.FileOriginal = name

	p.FileSize = int64(len(buf))

	imgConfig, _, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("unsupported filetype")
	}
	p.FileWidth, p.FileHeight = imgConfig.Width, imgConfig.Height
	return nil
}

func (p *Post) createThumbnail(b *Board, buf []byte, mimeType string, thumbPath string) error {
	thumbImg, err := resizeImage(b, bytes.NewReader(buf), mimeType)
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

func (p *Post) loadForm(r *http.Request, b *Board, rootDir string) error {
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

	mimeType := http.DetectContentType(buf)

	fileExt := mimeToExt(mimeType)
	if fileExt == "" {
		return fmt.Errorf("unsupported filetype")
	}

	p.setFileAndThumb(fileExt)

	err = p.setFileAttributes(buf, formFileHeader.Filename)
	if err != nil {
		return err
	}

	srcPath := filepath.Join(rootDir, b.Dir, "src", p.File)
	thumbPath := filepath.Join(rootDir, b.Dir, "thumb", p.Thumb)

	err = p.createThumbnail(b, buf, mimeType, thumbPath)
	if err != nil {
		return err
	}

	err = os.WriteFile(srcPath, buf, 0600)
	if err != nil {
		log.Fatal(err)
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

func (p *Post) ExpandHTML(b *Board) template.HTML {
	if p.File == "" {
		return ""
	} else if p.IsEmbed() {
		return template.HTML(p.File)
	}
	srcPath := fmt.Sprintf("%ssrc/%s", b.Path(), p.File)

	const expandFormat = `<a href="%s" onclick="return expandFile(event, '%d');"><img src="%s" width="%d" style="min-width: %dpx;min-height: %dpx;max-width: 85vw;height: auto;"></a>`
	return template.HTML(url.PathEscape(fmt.Sprintf(expandFormat, srcPath, p.ID, srcPath, p.FileWidth, p.ThumbWidth, p.ThumbHeight)))
}

func (p *Post) RefLink(b *Board) template.HTML {
	return template.HTML(fmt.Sprintf(`<a href="%sres/%d.html#%d">&gt;&gt;%d</a>`, b.Path(), p.Thread(), p.ID, p.ID))
}

func mimeToExt(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
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
	case "image/jpeg":
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
