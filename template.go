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
	data.Opt = &srirachaServer.opt

	if strings.HasPrefix(data.Template, "board_") {
		prefix := "imgboard_"
		if data.Board != nil && data.Board.Type == TypeForum {
			prefix = "forum_"
		}
		data.Template = prefix + strings.TrimPrefix(data.Template, "board_")
	}

	err := srirachaServer.tpl.ExecuteTemplate(w, data.Template+".gohtml", data)
	if err != nil {
		log.Fatal(err)
	}
}

var templateFuncMap = template.FuncMap{
	"HTML": func(text string) template.HTML {
		return template.HTML(text)
	},
	"Omitted": func(showReplies int, threadPosts int) int {
		numReplies := threadPosts - 1
		if showReplies == 0 || numReplies <= showReplies {
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
	"T": func(message string, vars ...interface{}) string {
		return gotext.Get(message, vars...)
	},
	"TN": func(singular string, plural string, n int, vars ...interface{}) string {
		return gotext.GetN(singular, plural, n, vars...)
	},
	"Title": strings.Title,
	"ZeroPadTo3": func(i int) string {
		return fmt.Sprintf("%03d", i)
	},
	"URLEscape": func(text string) string {
		return url.PathEscape(text)
	},
}

var guestData = &templateData{
	Manage: &manageData{},
}
