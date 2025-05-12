package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

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

	if strings.HasSuffix(r.URL.Path, "/attachment") {
		text := `[<a href="./">Return</a>]<br>
		<table class="managetable">
			<tr>
				<th align="left">Attachment</th>
				<th align="left">Posts</th>
				<th align="left">Size</th>
			</tr>`
		postStats := make(map[string]int64)
		sizeStats := make(map[string]int64)
		for _, b := range boards {
			threads := db.AllThreads(b, false)
			for _, thread := range threads {
				for _, post := range db.AllPostsInThread(thread.ID, false) {
					if post.IsEmbed() {
						postStats[post.EmbedInfo()[1]]++
					} else if post.File != "" {
						ext := filepath.Ext(post.File)
						if ext != "" {
							ext = strings.ToUpper(ext[1:])
							postStats[ext]++
							sizeStats[ext] += post.FileSize
						}
					}
				}
			}
		}
		extensions := make([]string, len(postStats))
		var i int
		for ext := range postStats {
			extensions[i] = ext
			i++
		}
		sort.Strings(extensions)
		for _, ext := range extensions {
			text += fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%s</td>", ext, postStats[ext], sriracha.FormatFileSize(sizeStats[ext]))
		}
		text += "</table>"
		return text, nil
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
	text += `</table><br><form method="get" action="statistics/attachment"><input type="submit" value="View Attachment Statistics"></form><br>`
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
