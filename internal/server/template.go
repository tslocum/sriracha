package server

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"image/png"
	"io"
	"log"
	"maps"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"codeberg.org/tslocum/gotext"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/pquerna/otp/totp"
)

//go:embed template
var templateFS embed.FS

type manageData struct {
	Account    *Account
	Accounts   []*Account
	Ban        *Ban
	Bans       []*Ban
	Banner     *Banner
	Banners    []*Banner
	Board      *Board
	Boards     []*Board
	Category   *Category
	Keyword    *Keyword
	Keywords   []*Keyword
	LiftedBans []*Ban
	Log        *Log
	Logs       []*Log
	News       *News
	AllNews    []*News
	Page       *Page
	Pages      []*Page
	Plugin     *pluginInfo
	Plugins    []*pluginInfo
	Report     *Report
	Reports    []*Report
	Threshold  *Threshold
	Thresholds []*Threshold
	TwoFactor  *TwoFactor
	TwoFactors []*TwoFactor
}

type templateData struct {
	Account       *Account
	Info          string
	Message       template.HTML
	Message2      template.HTML
	Message3      template.HTML
	Board         *Board
	Boards        []*Board
	Categories    []*Category
	News          *News
	AllNews       []*News
	Subscriptions []*Subscription
	Page          int
	Pages         int
	Post          *Post
	Threads       [][]*Post
	ReplyMode     int
	ModMode       bool
	Preview       bool
	Extra         string
	Extra2        string
	Extra3        string
	Opt           *ServerOptions
	Manage        *manageData
	Template      string

	// Calculated fields.
	IndexBoards      []*Board
	tpl              *template.Template
	buf              *bytes.Buffer
	templateFuncMaps map[string]template.FuncMap
}

func (data *templateData) Style() string {
	switch {
	case data.Account != nil:
		return data.Account.Style
	case data.Board != nil:
		return data.Board.Style
	default:
		return ""
	}
}

func (data *templateData) ManageMode() bool {
	return strings.HasPrefix(data.Template, "manage_")
}

func (data *templateData) GuideLink() string {
	return `<a href="/guide.html" target="_blank">` + Get(data.Board, data.Account, "visitor guide") + `</a>`
}

func (data *templateData) BoardError(w http.ResponseWriter, message string) {
	data.Template = "board_error"
	data.Info = message
	data.execute(w)
}

func (data *templateData) ManageError(message string) {
	data.Template = "manage_error"
	data.Info = message
}

func (data *templateData) forbidden(w http.ResponseWriter, required AccountRole) bool {
	allow := required != 0 && data.Account != nil && data.Account.Role != 0 && data.Account.Role <= required
	if allow {
		return false
	}
	data.Template = "manage_error"
	data.Info = "Access forbidden."
	return true
}

func (data *templateData) Redirect(w http.ResponseWriter, r *http.Request, destination string) {
	data.Template = ""
	http.Redirect(w, r, destination, http.StatusFound)
}

func (data *templateData) G(str string) string {
	return G(data.Board, data.Account, str)
}

func (data *templateData) Get(str string, vars ...interface{}) string {
	return Get(data.Board, data.Account, str, vars...)
}

func (data *templateData) GetHTML(str string, vars ...interface{}) template.HTML {
	return GetHTML(data.Board, data.Account, str, vars...)
}

func (data *templateData) GetN(singular string, plural string, v int) string {
	return GetN(data.Board, data.Account, singular, plural, v)
}

func (data *templateData) executeWithError(w io.Writer) error {
	if data.Template == "" {
		return nil
	}

	if data.Account != nil {
		data.IndexBoards = data.Boards
	} else {
		data.IndexBoards = data.IndexBoards[:0]
		for _, b := range data.Boards {
			if b.Hide == HideIndex || b.Hide == HideEverywhere {
				continue
			}
			data.IndexBoards = append(data.IndexBoards, b)
		}
	}

	var boardTemplate bool
	if strings.HasPrefix(data.Template, "board_") {
		prefix := "imgboard_"
		if data.Board != nil && data.Board.Type == TypeForum {
			prefix = "forum_"
		}
		data.Template = prefix + strings.TrimPrefix(data.Template, "board_")
		boardTemplate = true
	}

	responseWriter, ok := w.(http.ResponseWriter)
	if ok {
		responseWriter.Header().Set("Content-Type", "text/html")
	}

	var funcMap template.FuncMap
	if strings.HasPrefix(data.Template, "manage_") && data.Account != nil && data.Account.Locale != "" {
		funcMap = data.templateFuncMaps[data.Account.Locale]
	} else if boardTemplate {
		var locale string
		if data.Account != nil {
			locale = data.Account.Locale
		} else if data.Board != nil {
			locale = data.Board.Locale
		}
		funcMap = data.templateFuncMaps[locale]
	}
	if funcMap == nil {
		funcMap = data.templateFuncMaps[""]
	}

	data.buf.Reset()

	tplName := data.Template + ".gohtml"
	if data.Template == "line" {
		tplName = data.Template
	}
	err := data.tpl.Funcs(funcMap).ExecuteTemplate(data.buf, tplName, data)
	if err != nil {
		return err
	}

	io.Copy(w, data.buf)
	return nil
}

func (data *templateData) execute(w io.Writer) {
	err := data.executeWithError(w)
	if err != nil {
		log.Fatal(err)
	}
}

var expandableMedia = []string{".bmp", ".gif", ".jpg", ".png", ".svg", ".tif"}

var templateFuncMap = template.FuncMap{
	"Add64": func(a int64, b int64) int64 {
		return a + b
	},
	"Banner": func(banners []*Banner) *Banner {
		l := len(banners)
		switch l {
		case 0:
			return nil
		case 1:
			return banners[0]
		default:
			return banners[rand.Intn(l)]
		}
	},
	"Contains": strings.Contains,
	"Div": func(i int64, j int64) float64 {
		return float64(i) / float64(j)
	},
	"FormatDateInput": FormatDateInput,
	"FormatYYMMDD":    FormatYYYYMMDD,
	"Format": func(text string) template.HTML {
		return template.HTML(strings.ReplaceAll(text, "\n", "<br>\n"))
	},
	"GetBoard": func(boardID int, boards []*Board) *Board {
		for _, board := range boards {
			if board.ID == boardID {
				return board
			}
		}
		return nil
	},
	"HasExpandableMedia": func(thread []*Post) bool {
		for _, p := range thread {
			if p.File != "" && !p.IsEmbed() && slices.Contains(expandableMedia, filepath.Ext(p.File)) {
				return true
			}
		}
		return false
	},
	"HasPrefix": strings.HasPrefix,
	"HasSuffix": strings.HasSuffix,
	"HTML": func(text string) template.HTML {
		return template.HTML(text)
	},
	"Iterate": func(i int) []int {
		var values []int
		for v := 0; v <= i; v++ {
			values = append(values, v)
		}
		return values
	},
	"May": func(action string, account *Account, access map[string]string) bool {
		var required AccountRole
		switch access[action] {
		case "mod":
			required = RoleMod
		case "admin":
			required = RoleAdmin
		case "super-admin":
			required = RoleSuperAdmin
		default:
			return false
		}
		return account != nil && account.Role <= required
	},
	"MinusOne": func(i int) int {
		return i - 1
	},
	"Now": func() int64 {
		return time.Now().Unix()
	},
	"Omitted": func(showReplies int, numReplies int) int {
		if showReplies == 0 {
			return numReplies
		} else if numReplies <= showReplies {
			return 0
		}
		return numReplies - showReplies
	},
	"PlusOne": func(i int) int {
		return i + 1
	},
	"ShowReply": func(showReplies int, threadPosts int, postIndex int) bool {
		if showReplies == 0 {
			return true
		}
		return postIndex >= threadPosts-showReplies
	},
	"Slice": func(elements ...any) []any {
		return elements
	},
	"Sprintf": fmt.Sprintf,
	"ToUpper": strings.ToUpper,
	"ToLower": strings.ToLower,
	"Title":   strings.Title,
	"UnderscoreTitle": func(text string) string {
		return strings.Title(strings.ReplaceAll(text, "_", " "))
	},
	"URLEscape": func(text string) string {
		return url.PathEscape(text)
	},
	"ZeroPadTo3": func(i int) string {
		return fmt.Sprintf("%03d", i)
	},
}

func (s *Server) newTemplateFuncMap(locale string) template.FuncMap {
	f := make(template.FuncMap)
	maps.Copy(f, templateFuncMap)

	domain := Domain(locale)

	// Global settings.
	f["Global"] = func(setting string) bool {
		return slices.Contains(s.opt.Global, setting)
	}
	f["Globe"] = func(setting string) template.HTML {
		if slices.Contains(s.opt.Global, setting) {
			return template.HTML(`<div class="globe"> <span title="` + gotext.GetD(domain, "Global") + `">🌐</span></div>`)
		}
		return ""
	}

	// Localization.
	f["T"] = func(message string, vars ...interface{}) string {
		return gotext.GetD(domain, message, vars...)
	}
	f["TN"] = func(singular string, plural string, n int) string {
		return gotext.GetND(domain, singular, plural, n, n)
	}

	// Help icon and link.
	f["Help"] = func(anchor string) template.HTML {
		return template.HTML(fmt.Sprintf(`<div class="managehelp" title="%s"><a href="https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#%s" target="_blank">📖</a></div>`, html.EscapeString(gotext.GetD(domain, "View Sriracha manual subsection")), html.EscapeString(anchor)))
	}

	// Ban.
	f["AllActiveBans"] = func(rangeOnly bool) []*Ban { return s.tplDB.AllActiveBans(rangeOnly) }
	f["LiftedBansByIP"] = func(ip string) []*Ban { return s.tplDB.LiftedBansByIP(ip) }
	f["BanByID"] = func(id int) *Ban { return s.tplDB.BanByID(id) }
	f["BanByIP"] = func(ip string) *Ban { return s.tplDB.BanByIP(ip) }

	// Board.
	f["BoardByID"] = func(id int) *Board { return s.tplDB.BoardByID(id) }
	f["BoardByDir"] = func(dir string) *Board { return s.tplDB.BoardByDir(dir) }
	f["UniqueUserPosts"] = func(b *Board) int { return s.tplDB.UniqueUserPosts(b) }
	f["AllBoards"] = func() []*Board { return s.tplDB.AllBoards() }

	// News.
	f["NewsByID"] = func(id int) *News { return s.tplDB.NewsByID(id) }
	f["AllNews"] = func(onlyPublished bool) []*News { return s.tplDB.AllNews(onlyPublished) }

	// Page.
	f["PageByID"] = func(id int) *Page { return s.tplDB.PageByID(id) }
	f["PageByPath"] = func(path string) *Page { return s.tplDB.PageByPath(path) }
	f["AllPages"] = func() []*Page { return s.tplDB.AllPages() }

	// Post.
	f["AllThreads"] = func(board *Board, moderated bool) [][2]int { return s.tplDB.AllThreads(board, moderated) }
	f["AllPostsInThread"] = func(postID int, moderated bool) []*Post { return s.tplDB.AllPostsInThread(postID, moderated) }
	f["AllReplies"] = func(threadID int, limit int, moderated bool) []*Post {
		return s.tplDB.AllReplies(threadID, limit, moderated)
	}
	f["PendingPosts"] = func() []*Post { return s.tplDB.PendingPosts() }
	f["PostByID"] = func(postID int) *Post { return s.tplDB.PostByID(postID) }
	f["PostsByIP"] = func(hash string) []*Post { return s.tplDB.PostsByIP(hash) }
	f["PostsByFileHash"] = func(hash string, filterBoard *Board) []*Post { return s.tplDB.PostsByFileHash(hash, filterBoard) }
	f["PostByField"] = func(board *Board, field string, value any) *Post { return s.tplDB.PostByField(board, field, value) }
	f["LastPostByIP"] = func(board *Board, ip string) *Post { return s.tplDB.LastPostByIP(board, ip) }
	f["ReplyCount"] = func(threadID int) int { return s.tplDB.ReplyCount(threadID) }

	// Two-factor authentication.
	f["TOTPImage"] = func(a *Account, t *TwoFactor) template.HTML {
		buf := &bytes.Buffer{}
		buf.WriteString(`<img src="data:image/png;base64,`)
		encoder := base64.NewEncoder(base64.StdEncoding, buf)
		key, err := totp.Generate(s.twoFactorOptions(a, t))
		if err != nil {
			log.Fatal(err)
		}
		img, err := key.Image(totpImageSize, totpImageSize)
		if err != nil {
			log.Fatal(err)
		}
		err = png.Encode(encoder, img)
		if err != nil {
			log.Fatal(err)
		}
		encoder.Close()
		fmt.Fprintf(buf, `" width="%d" height="%d" alt="QR code">`, totpImageSize, totpImageSize)
		return template.HTML(buf.String())
	}
	return f
}

func (s *Server) newTemplateData(db serverDB) *templateData {
	if db != nil {
		s.tplDB = db
	}
	const initialBufferSize = 128000 // 128 Kilobytes.
	writeBuf := bytes.NewBuffer(make([]byte, initialBufferSize))
	return &templateData{
		Manage: &manageData{
			Plugins: allPluginInfo,
		},
		Opt:              &s.opt,
		tpl:              s.tpl,
		buf:              writeBuf,
		templateFuncMaps: s.templateFuncMaps,
	}
}
