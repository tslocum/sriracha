package server

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/leonelquinteros/gotext"
)

//go:embed template
var templateFS embed.FS

type manageData struct {
	Account  *Account
	Accounts []*Account
	Ban      *Ban
	Bans     []*Ban
	Banner   *Banner
	Banners  []*Banner
	Board    *Board
	Boards   []*Board
	Category *Category
	Keyword  *Keyword
	Keywords []*Keyword
	Log      *Log
	Logs     []*Log
	News     *News
	AllNews  []*News
	Page     *Page
	Pages    []*Page
	Plugin   *pluginInfo
	Plugins  []*pluginInfo
	Report   *Report
	Reports  []*Report
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
	Extra         string
	Extra2        string
	Extra3        string
	Opt           *ServerOptions
	Manage        *manageData
	Template      string

	// Calculated fields.
	IndexBoards []*Board
	tpl         *template.Template
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
		funcMap = templateFuncMaps[data.Account.Locale]
	} else if boardTemplate {
		var locale string
		if data.Account != nil {
			locale = data.Account.Locale
		} else if data.Board != nil {
			locale = data.Board.Locale
		}
		funcMap = templateFuncMaps[locale]
	}
	if funcMap == nil {
		funcMap = templateFuncMaps[""]
	}

	tplName := data.Template + ".gohtml"
	if data.Template == "line" {
		tplName = data.Template
	}
	return data.tpl.Funcs(funcMap).ExecuteTemplate(w, tplName, data)
}

func (data *templateData) execute(w io.Writer) {
	err := data.executeWithError(w)
	if err != nil {
		log.Fatal(err)
	}
}

var expandableMedia = []string{".bmp", ".gif", ".jpg", ".png", ".svg", ".tif"}

var templateFuncMap = template.FuncMap{
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

var templateFuncMaps map[string]template.FuncMap

func newTemplateFuncMap(locale string) template.FuncMap {
	f := make(template.FuncMap)
	for name, v := range templateFuncMap {
		f[name] = v
	}

	domain := "sriracha"
	if locale != "" {
		domain += "-" + locale
	}
	f["T"] = func(message string, vars ...interface{}) string {
		return gotext.GetD(domain, message, vars...)
	}
	f["TN"] = func(singular string, plural string, n int, vars ...interface{}) string {
		return gotext.GetND(domain, singular, plural, n, vars...)
	}
	return f
}

func (s *Server) newTemplateData() *templateData {
	return &templateData{
		Manage: &manageData{
			Plugins: allPluginInfo,
		},
		Opt: &s.opt,
		tpl: s.tpl,
	}
}
