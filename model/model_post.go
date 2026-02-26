package model

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"hash/adler32"
	"html"
	"html/template"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	. "codeberg.org/tslocum/sriracha/util"
)

var (
	adler    = adler32.New()
	adlerBuf = make([]byte, 8)
	adlerSum []byte
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
	FileMIME     string
	FileHash     string
	FileOriginal string
	FileSize     int64
	FileWidth    int
	FileHeight   int
	Thumb        string
	ThumbWidth   int
	ThumbHeight  int
	Moderated    PostModerated
	Stickied     bool
	Locked       bool

	// Calculated fields.
	Replies int
}

func (p *Post) Copy() *Post {
	pp := &Post{}
	*pp = *p
	pp.Board = p.Board
	return pp
}

func (p *Post) ResetAttachment() {
	p.File = ""
	p.FileMIME = ""
	p.FileHash = ""
	p.FileOriginal = ""
	p.FileSize = 0
	p.FileWidth = 0
	p.FileHeight = 0
	p.Thumb = ""
	p.ThumbWidth = 0
	p.ThumbHeight = 0
}

func (p *Post) AddMediaOverlay(img image.Image) image.Image {
	mediaBuf, err := os.ReadFile("static/img/media.png")
	if err != nil {
		log.Fatal(err)
	}

	overlayImg, err := png.Decode(bytes.NewReader(mediaBuf))
	if err != nil {
		log.Fatal(err)
	}

	target := image.NewRGBA(img.Bounds())
	draw.Draw(target, img.Bounds(), img, image.Point{}, draw.Src)

	overlayPosition := image.Point{
		X: img.Bounds().Dx()/2 - overlayImg.Bounds().Dx()/2,
		Y: img.Bounds().Dy()/2 - overlayImg.Bounds().Dy()/2,
	}
	draw.Draw(target, overlayImg.Bounds().Add(overlayPosition), overlayImg, image.Point{}, draw.Over)
	return target
}

func (p *Post) SetNameBlock(defaultName string, capcode string, identifiers bool) {
	var out strings.Builder

	emailLink := p.Email != "" && strings.ToLower(p.Email) != "noko"

	if emailLink {
		out.WriteString(`<a href="mailto:` + html.EscapeString(p.Email) + `">`)
	}
	if p.Name != "" || p.Tripcode == "" {
		name := p.Name
		if name == "" {
			if strings.ContainsRune(defaultName, '|') {
				split := strings.Split(defaultName, "|")
				name = split[rand.Intn(len(split))]
			} else {
				name = defaultName
			}
		}
		out.WriteString(`<span class="postername">`)
		out.WriteString(html.EscapeString(name))
		out.WriteString(`</span>`)
	}
	if p.Tripcode != "" {
		out.WriteString(`<span class="postertrip">!`)
		out.WriteString(html.EscapeString(p.Tripcode))
		out.WriteString(`</span>`)
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

	identifier := p.Identifier(identifiers, false)
	if identifier != "" {
		out.WriteString(" " + identifier)
	}

	out.WriteString(" " + string(p.TimestampLabel()))

	p.NameBlock = out.String()
}

func (p *Post) Thread() int {
	if p.Parent == 0 {
		return p.ID
	}
	return p.Parent
}

func (p *Post) FileSizeLabel() string {
	return FormatFileSize(p.FileSize)
}

func (p *Post) TimestampLabel() template.HTML {
	return FormatTimestamp(p.Timestamp)
}

func (p *Post) IsOekaki() bool {
	return strings.HasSuffix(p.File, ".tgkr")
}

func (p *Post) IsSWF() bool {
	return strings.HasSuffix(p.File, ".swf")
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

func (p *Post) MessageTruncated(lines int, account *Account) template.HTML {
	var showOmitted bool
	if lines == 0 {
		lines = p.Board.Truncate
		showOmitted = true
	}
	if lines == 0 {
		return template.HTML(p.Message)
	}

	count := strings.Count(p.Message, "\n")
	if count < lines {
		return template.HTML(p.Message)
	}

	msg := []byte(p.Message)
	out := &bytes.Buffer{}
	var start int
	for i := 0; i < lines; i++ {
		index := bytes.Index(msg[start:], []byte("\n"))

		end := len(msg) - start
		if index != -1 {
			end = index
		}

		if i > 0 {
			out.Write([]byte("\n"))
		}
		out.Write(msg[start : start+end])

		start += end + 4

		if start >= len(msg) {
			break
		}
	}

	buf := out.Bytes()
	buf = bytes.TrimSuffix(buf, []byte("\n"))
	buf = bytes.TrimSuffix(buf, []byte("<br>"))

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(buf))
	if err != nil {
		log.Fatal(err)
	}
	truncated, err := doc.Find("body").First().Html()
	if err != nil {
		log.Fatal(err)
	}

	if showOmitted {
		truncated += `<br><span class="omittedposts">` + Get(p.Board, account, "Post truncated. Click Reply to view.") + `</span><br>`
	}
	return template.HTML(truncated)
}

func (p *Post) ExpandHTML() template.HTML {
	if p.File == "" {
		return ""
	} else if p.IsEmbed() {
		return template.HTML(url.PathEscape(p.File))
	}
	srcPath := fmt.Sprintf("%ssrc/%s", p.Board.Path(), p.File)

	isAudio := strings.HasPrefix(p.FileMIME, "audio/")
	isVideo := strings.HasPrefix(p.FileMIME, "video/")
	if isAudio || isVideo {
		element := "audio"
		loop := ""
		if isVideo {
			element = "video"
			loop = " loop"
		}
		const expandFormat = `<%s width="%d" height="%d" class="thumb" style="pointer-events: inherit;" controls autoplay%s><source src="%s"></source></%s>`
		return template.HTML(url.PathEscape(fmt.Sprintf(expandFormat, element, p.FileWidth, p.FileHeight, loop, srcPath, element)))
	}

	isImage := strings.HasPrefix(p.FileMIME, "image/")
	if !isImage {
		return ""
	}
	const expandFormat = `<a href="%s" onclick="return expandFile(event, '%d');"><img src="%s" width="%d" height="%d" class="thumb" style="pointer-events: inherit;"></a>`
	return template.HTML(url.PathEscape(fmt.Sprintf(expandFormat, srcPath, p.ID, srcPath, p.FileWidth, p.FileHeight)))
}

func (p *Post) Identifier(identifiers bool, force bool) string {
	if p.IP == "" || !identifiers || (p.Board.Identifiers == IdentifiersDisable && !force) {
		return ""
	}
	adler.Reset()
	if p.Board.Identifiers == IdentifiersBoard {
		adler.Write([]byte(strconv.Itoa(p.Board.ID)))
	}
	adler.Write([]byte(p.IP))

	adlerSum = adler.Sum(adlerSum[:0])

	base64.RawURLEncoding.Encode(adlerBuf, adlerSum)
	return string(adlerBuf[:5])
}

func (p *Post) Backlinks(posts []*Post) template.HTML {
	if !p.Board.Backlinks {
		return ""
	}
	var out []byte
BACKLINKS:
	for _, reply := range posts {
		matches := RefLinkPattern.FindAll([]byte(reply.Message), -1)
		for _, match := range matches {
			id, err := strconv.Atoi(string(match)[8:])
			if err != nil || id != p.ID {
				continue
			} else if out != nil {
				out = append(out, []byte("<wbr>")...)
			}
			out = append(out, FormatRefLink(p.Board.Path(), p.Thread(), reply.ID)...)
			continue BACKLINKS
		}
	}
	return template.HTML(string(out))
}

func FormatRefLink(boardPath string, threadID int, postID int) []byte {
	return fmt.Appendf(nil, `<a href="%sres/%d.html#%d">&gt;&gt;%d</a>`, boardPath, threadID, postID, postID)
}

func (p *Post) RefLink() template.HTML {
	return template.HTML(FormatRefLink(p.Board.Path(), p.Thread(), p.ID))
}

func (p *Post) URL(siteHome string) string {
	var host string
	var path string
	if siteHome != "" {
		u, err := url.Parse(siteHome)
		if err == nil {
			if u.Host != "" {
				host = "https://" + u.Host
			}
			path = u.Path
		}
	}

	path = filepath.Join(path, p.Board.Path())
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	return fmt.Sprintf(`%s%sres/%d.html#%d`, host, path, p.Thread(), p.ID)
}
