package server

import (
	"fmt"
	"io"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

// newTestServer returns a new Server which is used only for testing.
// Database methods are mocked.
func newTestServer() (*Server, error) {
	s := NewServer()
	s.config = &Config{}

	err := s.loadServerConfig()
	if err != nil {
		return nil, err
	}

	err = s.parseTemplates("", s.config.Template, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template files: %s", err)
	}
	return s, nil
}

// validateTemplates executes loaded templates using dummy data.
func (s *Server) validateTemplates(ts *Server) error {
	if ts == nil {
		var err error
		ts, err = newTestServer()
		if err != nil {
			return err
		}
		ts.tpl = s.tpl
		ts.tplOriginal = s.tplOriginal
	}
	db := ts.begin()
	allBoards := db.AllBoards()
	img := db.BoardByDir("img")
	forum := db.BoardByDir("forum")

	var board *Board
	for _, c := range testCases {
		boardTemplate := strings.HasPrefix(c.template, "board_")
		for i := 0; i < 2; i++ {
			templateName := c.template
			if boardTemplate {
				if i == 0 {
					templateName = "imgboard_" + c.template[6:]
				} else {
					templateName = "forum_" + c.template[6:]
				}
			}

			forumTemplate := strings.HasPrefix(templateName, "forum_")
			if forumTemplate {
				board = forum
			} else {
				board = img
			}

			for j := 0; j < 3; j++ {
				data := ts.newTemplateData(db)
				data.Categories = db.AllCategories()
				data.Template = templateName
				if c.board {
					data.Board = board
				}
				data.Boards = allBoards
				if c.thread || c.threads {
					for _, thread := range db.AllThreads(board, true) {
						data.Threads = append(data.Threads, db.AllPostsInThread(thread[0], true))
						if c.thread {
							break
						}
					}
				}
				if c.thread {
					data.ReplyMode = 1
				}

				if strings.HasPrefix(templateName, "manage_") {
					data.Account = &Account{
						ID:       1,
						Username: "admin",
						Role:     RoleSuperAdmin,
					}
				}
				if c.manageBanner {
					data.Manage.Banner = &Banner{}
					for add := 0; add < j; add++ {
						data.Manage.Banners = append(data.Manage.Banners, data.Manage.Banner)
					}
				}
				if c.manageBoard {
					data.Manage.Board = board
					for add := 0; add < j; add++ {
						data.Manage.Boards = append(data.Manage.Boards, data.Manage.Board)
					}
				}
				if c.manageCategory {
					data.Manage.Category = data.Categories[0]
					for add := 0; add < j; add++ {
						data.Categories = append(data.Categories, data.Manage.Category)
					}
				}
				if c.manageKeyword {
					data.Manage.Keyword = &Keyword{
						ID:     1,
						Text:   "keyword",
						Action: "hide",
						Boards: allBoards,
					}
					for add := 0; add < j; add++ {
						data.Manage.Keywords = append(data.Manage.Keywords, data.Manage.Keyword)
					}
				}
				if c.managePage {
					data.Manage.Page = &Page{
						ID:      1,
						Path:    "path",
						Content: "content",
					}
					for add := 0; add < j; add++ {
						data.Manage.Pages = append(data.Manage.Pages, data.Manage.Page)
					}
				}
				if c.manageThreshold {
					data.Manage.Threshold = &Threshold{
						ID:       1,
						Amount:   1,
						Duration: 30,
						Action:   "delete",
					}
					for add := 0; add < j; add++ {
						data.Manage.Thresholds = append(data.Manage.Thresholds, data.Manage.Threshold)
					}
				}

				err := data.executeWithError(io.Discard)
				if err != nil {
					return fmt.Errorf("failed to execute template %s: %s", data.Template, err)
				}
			}

			if !boardTemplate {
				break
			}
		}
	}
	db.Commit()
	return nil
}

// testCase represents a template test case.
type testCase struct {
	template        string
	board           bool
	thread          bool
	threads         bool
	manageBanner    bool
	manageBoard     bool
	manageCategory  bool
	manageKeyword   bool
	managePage      bool
	manageThreshold bool
}

// testCases represents all template test cases.
var testCases = []testCase{
	{
		template: "board_error",
		board:    true,
	},
	{
		template: "board_info",
		board:    true,
	},
	{
		template: "board_page",
		board:    true,
		threads:  true,
	},
	{
		template: "board_page",
		board:    true,
		thread:   true,
	},
	{
		template: "board_post",
		board:    true,
		threads:  true,
	},
	{
		template: "guide",
		board:    true,
	},
	{
		template: "imgboard_catalog",
		board:    true,
		threads:  true,
	},
	{
		template: "index",
		board:    true,
	},
	{
		template: "news",
		board:    true,
	},
	{
		template: "oekaki",
		board:    true,
	},
	{
		template: "subscribe",
		board:    true,
	},
	{
		template: "manage_account",
	},
	{
		template: "manage_ban",
	},
	{
		template:     "manage_banner",
		manageBanner: true,
	},
	{
		template:    "manage_board",
		manageBoard: true,
	},
	{
		template:       "manage_category",
		manageCategory: true,
	},
	{
		template: "manage_error",
	},
	{
		template: "manage_info",
	},
	{
		template:      "manage_keyword",
		manageKeyword: true,
	},
	{
		template:      "manage_keyword_test",
		manageKeyword: true,
	},
	{
		template: "manage_log",
	},
	{
		template: "manage_login",
	},
	{
		template: "manage_mod",
		board:    true,
		threads:  true,
	},
	{
		template: "manage_news",
	},
	{
		template:   "manage_page",
		managePage: true,
	},
	{
		template: "manage_plugin",
	},
	{
		template: "manage_preference",
	},
	{
		template: "manage_setting",
	},
	{
		template: "manage_status",
	},
	{
		template:        "manage_threshold",
		manageThreshold: true,
	},
}
