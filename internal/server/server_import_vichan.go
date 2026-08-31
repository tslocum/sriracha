package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/dlclark/regexp2/v2"
	"github.com/dlclark/regexp2/v2/compat"
)

type vichanImport struct {
	db *sql.DB
}

type vichanFileInfo struct {
	Name      string
	FilePath  string `json:"file_path"`
	ThumbPath string `json:"thumb_path"`
}

func (v *vichanImport) Name() string {
	return "vichan"
}

func (v *vichanImport) tables() ([]string, error) {
	rows, err := v.db.Query("SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %s", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		err = rows.Scan(&table)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %s", err)
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list tables: %s", err)
	}
	return tables, nil
}

func (v *vichanImport) Matches() bool {
	tables, err := v.tables()
	if err != nil {
		return false
	}
	expected := []string{"antispam", "ban_appeals", "bans", "boards", "captchas", "cites", "flood", "ip_notes", "modlogs", "mods", "mutes", "news", "nntp_references", "noticeboard", "pages", "pms", "reports", "robot", "search_queries", "theme_settings"}
	for _, table := range expected {
		if !slices.Contains(tables, table) {
			return false
		}
	}
	return true
}

func (v *vichanImport) Tables() []string {
	var result []string
	tables, err := v.tables()
	if err != nil {
		log.Fatal(err)
	}
	for _, table := range tables {
		if strings.HasPrefix(table, "posts_") {
			result = append(result, table)
		}
	}
	return result
}

func (v *vichanImport) Posts(table string) []*Post {
	rows, err := v.db.Query("SELECT id, COALESCE(thread, 0), COALESCE(subject, ''), COALESCE(email, ''), COALESCE(name, ''), COALESCE(trip, ''), COALESCE(capcode, ''), COALESCE(body, ''), COALESCE(time, 0), COALESCE(bump, 0), COALESCE(files, ''), sticky, locked, COALESCE(embed, '') FROM " + table + " ORDER BY id ASC")
	if err != nil {
		log.Fatalf("failed to select posts: %s", err)
	}
	defer rows.Close()

	resPattern := compat.MustCompile(`<a onclick="highlightReply\('([0-9]+)', event\);" href="[^"]*res/([0-9]+).html#([0-9]+)">&gt;&gt;([0-9]+)</a>`)

	var pending []*Post

	var posts []*Post
	for rows.Next() {
		var (
			p = &Post{
				Moderated: ModeratedVisible,
			}
			capcode  string
			fileInfo []byte
			stickied int
			locked   int
			embed    string
		)
		err = rows.Scan(
			&p.ID,
			&p.Parent,
			&p.Subject,
			&p.Email,
			&p.Name,
			&p.Tripcode,
			&capcode,
			&p.Message,
			&p.Timestamp,
			&p.Bumped,
			&fileInfo,
			&stickied,
			&locked,
			&embed,
		)
		if err != nil {
			log.Fatalf("failed to scan post: %s", err)
		} else if len(fileInfo) > 0 {
			var allInfo []*vichanFileInfo
			err = json.Unmarshal(fileInfo, &allInfo)
			if err != nil {
				log.Fatalf("failed to scan files column: %s: %s", string(fileInfo), err)
			}
			for i, info := range allInfo {
				var current *Post
				if i == 0 {
					current = p
				} else {
					pp := p.Copy()
					pp.ID = 0
					if pp.Parent == 0 {
						pp.Parent = p.ID
					}
					pp.Subject = ""
					class := "refreply"
					if p.Parent == 0 {
						class = "refop"
					}
					pp.Message = fmt.Sprintf(`<a href="res/%d.html#%d" class="%s">&gt;&gt;%d</a>`, p.Thread(), p.ID, class, p.ID)
					pp.ResetAttachment()
					pending = append(pending, pp)
					current = pp
				}
				lastSlash := strings.LastIndexByte(info.FilePath, '/')
				if lastSlash == -1 {
					log.Fatalf("failed to parse file %s: no slash found in file path", string(fileInfo))
				}
				current.File = info.FilePath[lastSlash+1:]
				if current.File == "" {
					log.Fatalf("failed to parse file %s: blank file", string(fileInfo))
				}
				current.FileOriginal = info.Name
				lastSlash = strings.LastIndexByte(info.ThumbPath, '/')
				if lastSlash != -1 {
					current.Thumb = info.ThumbPath[lastSlash+1:]
				}
			}
		}
		p.Stickied = stickied == 1
		p.Locked = locked == 1

		// Replace line break tags.
		p.Message = strings.ReplaceAll(p.Message, `<br/>`, `<br>`+"\n")

		// Replace quote class.
		p.Message = strings.ReplaceAll(p.Message, `<span class="quote">`, `<span class="unkfunc">`)

		// Replace reflinks.
		p.Message = ReplaceAllStringFunc(resPattern, p.Message, func(match regexp2.Match) string {
			groups := match.Groups()
			postID := ParseInt(groups[1].String())
			threadID := ParseInt(groups[2].String())
			class := "refreply"
			if postID == threadID {
				class = "refop"
			}
			return fmt.Sprintf(`<a href="res/%d.html#%d" class="%s">&gt;&gt;%d</a>`, threadID, postID, class, postID)
		})

		if embed != "" && p.File == "" {
			p.FileHash = "e YouTube"
			p.FileOriginal = embed
		}

		p.SetNameBlock("Anonymous", capcode, false, false)

		posts = append(posts, p)
	}
	if err = rows.Err(); err != nil {
		log.Fatalf("failed to select posts: %s", err)
	}

	for _, post := range pending {
		posts = append(posts, post)
	}
	return posts
}
