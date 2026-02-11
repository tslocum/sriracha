package server

import (
	"fmt"
	"io"
	"testing"

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

func newTestThread(size int) []*Post {
	var id int
	newPost := func() *Post {
		id++
		return &Post{
			ID:      id,
			Subject: "Subject",
			Name:    "Anonymous",
			Message: "Message",
		}
	}
	posts := make([]*Post, size)
	for i := 0; i < size; i++ {
		posts[i] = newPost()
		if i != 0 {
			posts[i].Parent = 1
		}
	}
	return posts
}

func newTestServer() (*Server, error) {
	s := NewServer()
	s.config = &Config{}

	s.setDefaultServerConfig()

	err := s.parseTemplates("", s.config.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template files: %s", err)
	}
	return s, nil
}

// BenchmarkBoardThread benchmarks board index pages.
func BenchmarkBoardIndex(b *testing.B) {
	s, err := newTestServer()
	if err != nil {
		b.Fatal(err)
	}

	board := newTestBoard()

	data := s.newTemplateData()
	data.Template = "board_page"
	data.Board = board
	data.Boards = []*Board{board}

	for i := 0; i < 10; i++ {
		data.Threads = append(data.Threads, newTestThread(i+1))
	}

	for _, thread := range data.Threads {
		for _, post := range thread {
			post.Board = board
			post.SetNameBlock("Anonymous", "", false)
		}
	}

	// Warm caches.
	data.execute(io.Discard)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.execute(io.Discard)
	}
}

// BenchmarkBoardThread benchmarks board thread pages.
func BenchmarkBoardThread(b *testing.B) {
	s, err := newTestServer()
	if err != nil {
		b.Fatal(err)
	}

	board := newTestBoard()

	data := s.newTemplateData()
	data.Template = "board_page"
	data.Board = board
	data.Boards = []*Board{board}
	data.Threads = [][]*Post{newTestThread(100)}

	for _, thread := range data.Threads {
		for _, post := range thread {
			post.Board = board
			post.SetNameBlock("Anonymous", "", false)
		}
	}

	// Warm caches.
	data.execute(io.Discard)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.execute(io.Discard)
	}
}
