package server

import (
	"fmt"
	"io"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func newTestBoard() *Board {
	return &Board{
		ID:          1,
		Dir:         "test",
		Name:        "Test",
		Description: "Test board",
		Type:        TypeImageboard,
	}
}

func newTestServer() (*Server, error) {
	s := NewServer()
	s.config = &Config{}

	s.loadServerConfig()

	db := s.begin()
	allBoards := db.AllBoards()
	s.opt.Categories = []*categoryInfo{
		{Boards: allBoards},
	}
	var recent []*Post
	for _, board := range allBoards {
		recent = append(recent, db.LastPostByBoard(board))
	}
	s.opt.Categories[0].Recent = recent
	db.Commit()

	err := s.parseTemplates("", s.config.Template, nil)
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
	db.Commit()

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

			data := ts.newTemplateData(nil)
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

			err := data.executeWithError(io.Discard)
			if err != nil {
				return fmt.Errorf("failed to execute template %s: %s", data.Template, err)
			}
			if !boardTemplate {
				break
			}
		}
	}
	return nil
}

type testCase struct {
	template string
	board    bool
	thread   bool
	threads  bool
}

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
}
