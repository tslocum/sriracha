package sriracha

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/leonelquinteros/gotext"
)

//go:embed template
var templateFS embed.FS

type manageData struct {
	Account  *Account
	Accounts []*Account
	Ban      *Ban
	Bans     []*Ban
	Board    *Board
	Boards   []*Board
	Keyword  *Keyword
	Keywords []*Keyword
	Log      *Log
	Logs     []*Log
	News     *News
	AllNews  []*News
	Plugin   *pluginInfo
	Plugins  []*pluginInfo
	Report   *Report
	Reports  []*Report
}

type templateData struct {
	Account   *Account
	Info      string
	Message   template.HTML
	Message2  template.HTML
	Message3  template.HTML
	Board     *Board
	Boards    []*Board
	News      *News
	AllNews   []*News
	Page      int
	Pages     int
	Post      *Post
	Threads   [][]*Post
	ReplyMode int
	ModMode   bool
	Extra     string
	Opt       *ServerOptions
	Manage    *manageData
	Template  string
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
	allow := data.Account != nil && data.Account.Role != 0 && data.Account.Role <= required
	if allow {
		return false
	}
	data.Template = "manage_error"
	data.Info = "Access forbidden."
	return true
}

func (data *templateData) execute(w io.Writer) {
	if data.Template == "" {
		return
	}
	data.Opt = &srirachaServer.opt

	if strings.HasPrefix(data.Template, "board_") {
		prefix := "imgboard_"
		if data.Board != nil && data.Board.Type == TypeForum {
			prefix = "forum_"
		}
		data.Template = prefix + strings.TrimPrefix(data.Template, "board_")
	}

	responseWriter, ok := w.(http.ResponseWriter)
	if ok {
		responseWriter.Header().Set("Content-Type", "text/html")
	}

	tpl := srirachaServer.tpl
	var m template.FuncMap
	if strings.HasPrefix(data.Template, "manage_") && data.Account != nil && data.Account.Locale != "" {
		funcMap := templateFuncMaps[data.Account.Locale]
		if funcMap != nil {
			m = funcMap
		}
	} else {
		m = templateFuncMaps[""]
	}
	tpl = tpl.Funcs(m)

	err := tpl.ExecuteTemplate(w, data.Template+".gohtml", data)
	if err != nil {
		log.Fatal(err)
	}
}

var templateFuncMap = template.FuncMap{
	"Format": func(text string) template.HTML {
		return template.HTML(strings.ReplaceAll(text, "\n", "<br>\n"))
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

func newTemplateData() *templateData {
	return &templateData{
		Manage: &manageData{},
	}
}
