package statistics

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

type Statistics struct{}

func NewStatistics() *Statistics {
	return &Statistics{}
}

func (s *Statistics) About() string {
	return "View statistics for each board."
}

func (s *Statistics) Serve(db sriracha.DB, a *Account, w http.ResponseWriter, r *http.Request) (template.HTML, error) {
	boards := db.AllBoards()
	if len(boards) == 0 {
		return "No boards.", nil
	}

	if strings.HasSuffix(r.URL.Path, "/attachment") {
		text := `[<a href="./">Return</a>]<br>
		<table class="managetable">
			<tr>
				<th>Attachment</th>
				<th>Posts</th>
				<th>Size</th>
			</tr>`
		postStats := make(map[string]int64)
		sizeStats := make(map[string]int64)
		for _, b := range boards {
			allThreads := db.AllThreads(FilterAny, b)
			for _, threadInfo := range allThreads {
				for _, post := range db.AllPostsInThread(FilterAny, threadInfo[0]) {
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
			text += fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%s</td>", ext, postStats[ext], FormatFileSize(sizeStats[ext]))
		}
		text += "</table>"
		return template.HTML(text), nil
	}

	var totalPosts int
	var totalThreads int
	var totalSize int64
	text := `<table class="managetable">
        <tr>
            <th>Board</th>
            <th>Posts</th>
            <th>Threads</th>
            <th>Unique</th>
            <th>Attachments</th>
        </tr>`
	for _, b := range boards {
		allThreads := db.AllThreads(FilterAny, b)
		threadCount := len(allThreads)

		var postCount int
		var size int64
		for _, threadInfo := range allThreads {
			for _, post := range db.AllPostsInThread(FilterAny, threadInfo[0]) {
				postCount++
				size += post.FileSize
				totalSize += post.FileSize
			}
		}
		totalPosts += postCount
		totalThreads += threadCount
		b.Unique = db.UniqueUserPosts(b)
		text += fmt.Sprintf("<tr><td>%s %s</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td>", b.Path(), b.Name, postCount, threadCount, b.Unique, FormatFileSize(size))
	}
	text += fmt.Sprintf(`<tr><td>Total</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td>`, totalPosts, totalThreads, db.UniqueUserPosts(nil), FormatFileSize(totalSize))
	text += `</table><br><form method="get" action="statistics/attachment"><input type="submit" value="View Attachment Statistics"></form><br>`
	return template.HTML(text), nil
}

// Validate plugin interfaces during compilation.
var (
	_ sriracha.Plugin          = &Statistics{}
	_ sriracha.PluginWithServe = &Statistics{}
)
