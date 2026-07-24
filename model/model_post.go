package model

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"image"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	. "codeberg.org/tslocum/sriracha/util"
	"github.com/PuerkitoBio/goquery"
)

// audioMIME is a regular expression of an audio MIME type.
var audioMIME = regexp.MustCompile(`^audio/.*`)

// videoMIME is a regular expression of a video MIME type.
var videoMIME = regexp.MustCompile(`^video/.*`)

// imageMIME is a regular expression of an image MIME type.
var imageMIME = regexp.MustCompile(`^image/.*`)

// notExpandable is a regular expression of MIME types which lack wide support for inline playback.
var notExpandable = regexp.MustCompile(`^((audio/midi)|(video/(mpeg|ogg|x-matroska|x-msvideo)))$`)

type PostModerated int

const (
	ModeratedHidden   PostModerated = 0
	ModeratedVisible  PostModerated = 1
	ModeratedApproved PostModerated = 2
)

type PostBacklink struct {
	Source int
	Thread int
	Board  string
}

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
	Backlinks    []int

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

func (p *Post) PlainNameBlock(defaultName string, capcode string, identifiers bool) template.HTML {
	var out strings.Builder

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

	return template.HTML(out.String())
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

func (p *Post) BumpLabel() template.HTML {
	if p.Bumped != 0 {
		return FormatTimestamp(p.Bumped)
	}
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

	split := bytes.Split([]byte(p.Message), []byte("\n"))
	if len(split) <= lines {
		return template.HTML(p.Message)
	}

	blankMessage := template.HTML("…")
	if showOmitted {
		blankMessage = template.HTML(`<div class="omittedposts">` + Get(p.Board, account, "Post truncated. Click Reply to view.") + `</div><br>`)
	}

	buf := bytes.Join(split[:lines], []byte("\n"))
	if bytes.Contains(buf, []byte(`<div class="codeblock">`)) {
		return blankMessage
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(buf))
	if err != nil {
		log.Fatal(err)
	}
	body := doc.Find("body")
	if body == nil || body.Length() == 0 {
		return blankMessage
	}
	first := body.First()
	if first == nil || first.Length() == 0 || first.Text() == "" {
		return blankMessage
	}

	// Get document body HTML.
	truncated, err := first.Html()
	if err != nil {
		log.Fatal(err)
	}
	// Replace XHTML line break tags.
	truncated = strings.ReplaceAll(truncated, "<br/>", "<br>")

	if showOmitted {
		if !strings.HasSuffix(truncated, "<br>\n<br>") {
			truncated += "<br>"
		}
		truncated += string(blankMessage)
	}
	return template.HTML(truncated)
}

func (p *Post) InlineThumb(rootDir string) template.HTML {
	if p.Thumb == "" {
		return ""
	}
	out := &bytes.Buffer{}
	out.WriteString("data:")
	switch filepath.Ext(p.Thumb) {
	case ".jpg":
		out.WriteString("image/jpeg")
	case ".png":
		out.WriteString("image/png")
	case ".gif":
		out.WriteString("image/gif")
	default:
		return ""
	}
	out.WriteString(";base64,")
	f, err := os.Open(filepath.Join(rootDir, p.Board.Dir, "thumb", p.Thumb))
	if err != nil {
		log.Fatal(err)
	}
	encoder := base64.NewEncoder(base64.StdEncoding, out)
	_, err = io.Copy(encoder, f)
	if err != nil {
		log.Fatal(err)
	}
	encoder.Close()
	f.Close()
	return template.HTML(fmt.Sprintf(`<img src="%s" class="thumb" width="%d" height="%d">`, out.Bytes(), p.ThumbWidth, p.ThumbHeight))
}

func (p *Post) ExpandHTML() string {
	if p.File == "" {
		return ""
	} else if p.IsEmbed() {
		return p.File
	}
	srcPath := fmt.Sprintf("%ssrc/%s", p.Board.Path(), p.File)

	isAudio, isVideo := audioMIME.MatchString(p.FileMIME), videoMIME.MatchString(p.FileMIME)
	if (isAudio || isVideo) && !notExpandable.MatchString(p.FileMIME) {
		element := "audio"
		loop := ""
		if isVideo {
			element = "video"
			loop = " loop"
		}
		const expandFormat = `<%s width="%d" height="%d" style="pointer-events: inherit;" controls autoplay%s><source src="%s"></source></%s>`
		return fmt.Sprintf(expandFormat, element, p.FileWidth, p.FileHeight, loop, srcPath, element)
	}

	isImage := imageMIME.MatchString(p.FileMIME)
	if !isImage {
		return ""
	}
	const expandFormat = `<a href="%s" onclick="return expandFile(event, '%d');"><img src="%s" width="%d" height="%d" style="pointer-events: inherit;"></a>`
	return fmt.Sprintf(expandFormat, srcPath, p.ID, srcPath, p.FileWidth, p.FileHeight)
}

func (p *Post) Identifier(identifiers bool, force bool) string {
	if p.IP == "" || !identifiers || (p.Board.Identifiers == IdentifiersDisable && !force) {
		return ""
	}
	crcHash.Reset()
	if p.Board.Identifiers == IdentifiersBoard {
		crcHash.Write([]byte(strconv.Itoa(p.Board.ID)))
	}
	crcHash.Write([]byte(p.IP))

	crcSum = crcHash.Sum(crcSum[:0])

	base64.RawURLEncoding.Encode(crcBuf, crcSum)
	return string(crcBuf[:5])
}

func (p *Post) Mentions() []int {
	var mentions []int
	matches := RefLinkPattern.FindAll([]byte(p.Message), -1)
	for _, match := range matches {
		id, err := strconv.Atoi(string(match)[8:])
		if err != nil || id <= 0 {
			continue
		}
		mentions = append(mentions, id)
		continue
	}
	return mentions
}

func (p *Post) BacklinksLabel() template.HTML {
	if !p.Board.Backlinks || len(p.Backlinks) == 0 {
		return ""
	}
	var out []byte
	for _, backlinkID := range p.Backlinks {
		out = append(out, FormatRefLink(p.Board.Path(), p.Thread(), backlinkID)...)
	}
	return template.HTML(`<span class="backlink">` + string(out) + `</span>`)
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

func (p *Post) SearchText() string {
	subj := strings.TrimSpace(p.Subject)
	msg := strings.TrimSpace(p.Message)
	if msg != "" {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(msg)))
		if err != nil {
			log.Fatal(err)
		}
		msg = doc.Text()
	}
	if subj == "" && msg == "" {
		return ""
	} else if subj == "" {
		return msg
	} else if msg == "" {
		return subj
	}
	return subj + "\n" + msg
}
