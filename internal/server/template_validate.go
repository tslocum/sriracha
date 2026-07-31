package server

import (
	"fmt"
	"io"
	"runtime"
	"slices"
	"strings"
	"sync"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

// newTestServer returns a new Server which is used only for testing.
// Database methods are mocked.
func newTestServer() (*Server, error) {
	s := NewServer()
	s.config = &Config{
		Styles: []string{"futaba", "burichan", "sriracha"},
	}

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

func (s *Server) _validateTemplates(db serverDB, allBoards []*Board, img *Board, forum *Board, testCases chan testCase, wg *sync.WaitGroup, errors chan error, verbose bool) {
	wrapError := func(name string, err error) error {
		var source string
		if !slices.Contains(s.customTemplates, name) {
			source = "official"
		} else {
			source = "custom"
		}
		return fmt.Errorf("failed to execute %s template file %s: %s", source, name, err)
	}
	var board *Board
	for c := range testCases {
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
				data := s.newTemplateData()
				data.Categories = db.AllCategories()
				data.Template = templateName
				if c.board {
					data.Board = board
				}
				data.Boards = allBoards
				if c.thread || c.threads {
					for _, thread := range db.AllThreads(true, board) {
						data.Threads = append(data.Threads, db.AllPostsInThread(true, thread[0]))
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
				if c.manageTwoFactor {
					data.Manage.TwoFactor = &TwoFactor{
						ID:         1,
						Account:    1,
						Timestamp:  1,
						LastActive: 1,
						Secret:     "secret",
						Name:       "name",
					}
					for add := 0; add < j; add++ {
						data.Manage.TwoFactors = append(data.Manage.TwoFactors, data.Manage.TwoFactor)
					}
				}

				err := data.executeWithError(io.Discard)
				if err != nil {
					errors <- wrapError(data.Template, err)
					return
				}
			}

			if !boardTemplate {
				break
			}
		}
		if verbose {
			fmt.Print(".")
		}
		wg.Done()
	}
}

// validateTemplates executes loaded templates using dummy data.
func (s *Server) validateTemplates(ts *Server, verbose bool) error {
	if ts == nil {
		var err error
		ts, err = newTestServer()
		if err != nil {
			return err
		}
		ts.tpl = s.tpl
		ts.tplOriginal = s.tplOriginal
	}

	process := make(chan testCase)
	errors := make(chan error)
	wg := &sync.WaitGroup{}
	db := ts.begin()
	defer db.Commit()
	allBoards := db.AllBoards()
	img := db.BoardByDir("img")
	forum := db.BoardByDir("forum")

	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		go ts._validateTemplates(db, allBoards, img, forum, process, wg, errors, verbose)
	}

	wg.Add(len(testCases))
	for _, c := range testCases {
		select {
		case process <- c:
		case err := <-errors:
			close(process)
			return err
		}
	}
	close(process)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case err := <-errors:
		return err
	}
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
	manageTwoFactor bool
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
	{
		template:        "manage_twofactor",
		manageTwoFactor: true,
	},
}
