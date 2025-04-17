package sriracha

import (
	"embed"
	"html/template"
	"io"
	"log"
	"strings"
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
}

type templateData struct {
	Account   *Account
	Info      string
	Message   template.HTML
	Board     *Board
	Boards    []*Board
	Threads   [][]*Post
	ReplyMode int
	Manage    *manageData
	Template  string
}

func (data *templateData) Error(message string) {
	data.Template = "manage_error"
	data.Info = message
}

func (data *templateData) execute(w io.Writer) {
	err := srirachaServer.tpl.ExecuteTemplate(w, data.Template+".gohtml", data)
	if err != nil {
		log.Fatal(err)
	}
}

func withFuncMap(tpl *template.Template) *template.Template {
	funcMap := template.FuncMap{
		"Title": strings.Title,
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
	}
	return tpl.Funcs(funcMap)
}

var guestData = &templateData{
	Manage: &manageData{},
}
