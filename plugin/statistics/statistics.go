package main

import (
	"fmt"
	"net/http"

	"codeberg.org/tslocum/sriracha"
)

type Statistics struct{}

func (s *Statistics) About() string {
	return "View statistics for each board."
}

func (s *Statistics) Serve(db *sriracha.Database, a *sriracha.Account, w http.ResponseWriter, r *http.Request) (string, error) {
	boards := db.AllBoards()
	if len(boards) == 0 {
		return "No boards.", nil
	}

	var totalPosts int
	var totalThreads int
	var totalSize int64
	text := `<table class="managetable">
        <tr>
            <th align="left">Board</th>
            <th align="left">Posts</th>
            <th align="left">Threads</th>
            <th align="left">Unique</th>
            <th align="left">Attachments</th>
        </tr>`
	for _, b := range boards {
		threads := db.AllThreads(b, false)
		threadCount := len(threads)

		var postCount int
		var size int64
		for _, thread := range threads {
			for _, post := range db.AllPostsInThread(thread.ID, false) {
				postCount++
				size += post.FileSize
				totalSize += post.FileSize
			}
		}
		totalPosts += postCount
		totalThreads += threadCount
		b.Unique = db.UniqueUserPosts(b)
		text += fmt.Sprintf("<tr><td>%s %s</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td>", b.Path(), b.Name, postCount, threadCount, b.Unique, sriracha.FormatFileSize(size))
	}
	text += fmt.Sprintf(`<tr><td>Total</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td>`, totalPosts, totalThreads, db.UniqueUserPosts(nil), sriracha.FormatFileSize(totalSize))
	text += "</table>"
	return text, nil
}

func init() {
	sriracha.RegisterPlugin(&Statistics{})
}

func main() {}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin          = &Statistics{}
	_ sriracha.PluginWithServe = &Statistics{}
)
