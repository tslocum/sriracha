package sriracha

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sys/unix"
)

func (s *Server) serveImport(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_info"
	data.Message = `<h2 class="managetitle">Import</h2><b>Warning:</b> Backup all files and databases before importing a board.<br><br>`

	const completeMessage = "<b>Import complete.</b><br>Please remove the import option from config.yml and restart Sriracha.<br>"
	if data.forbidden(w, RoleSuperAdmin) {
		return
	} else if !s.config.importMode {
		data.ManageError("Sriracha is not running in import mode.")
		return
	} else if s.config.importComplete {
		data.Message += template.HTML(completeMessage)
		return
	}
	c := s.config.Import

	commit := formBool(r, "import") && formBool(r, "confirmation")
	defer func() {
		var command = "ROLLBACK"
		if commit && s.config.importComplete {
			command = "COMMIT"
			data.Message += template.HTML("Committing changes...<br>")
		}
		_, err := db.conn.Exec(context.Background(), command)
		if commit {
			if err != nil {
				data.Message += template.HTML("<b>Error:</b> Failed to commit changes: " + html.EscapeString(err.Error()))
			} else {
				data.Message += template.HTML("<b>Changes committed.</b><br><br>" + completeMessage)
			}
		}
	}()

	// Connect to the database.
	data.Message += template.HTML("Connecting to database...<br>")
	databaseURL := fmt.Sprintf("postgres://%s:%s@%s/%s", c.Username, c.Password, c.Address, c.DBName)
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		data.Message += template.HTML("<b>Error:</b> Failed to connect to database: " + html.EscapeString(err.Error()))
		return
	}
	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		data.Message += template.HTML("<b>Error:</b> Failed to verify connection status: " + html.EscapeString(err.Error()))
		return
	}
	data.Message += template.HTML("<b>Connected.</b><br><br>")

	// Validate tables.
	data.Message += template.HTML("Validating tables...<br>")
	tableEntries := func(name string) (int, error) {
		var entries int
		err := conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM "+name).Scan(&entries)
		if err == pgx.ErrNoRows {
			return 0, nil
		} else if err != nil {
			return 0, fmt.Errorf("failed to select from table %s: %s", name, err)
		}
		return entries, nil
	}

	postEntries, err := tableEntries(c.Posts)
	if err != nil {
		data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to validate table %s: %s", html.EscapeString(c.Posts), html.EscapeString(err.Error())))
		return
	} else if postEntries == 0 {
		data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> No posts were found in table %s.", html.EscapeString(c.Posts)))
		return
	}
	data.Message += template.HTML(fmt.Sprintf("<b>Found %d posts</b> in table %s.<br>", postEntries, html.EscapeString(c.Posts)))

	if c.Keywords != "" {
		keywordEntries, err := tableEntries(c.Keywords)
		if err != nil {
			data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to validate table %s: %s", html.EscapeString(c.Keywords), html.EscapeString(err.Error())))
			return
		}
		data.Message += template.HTML(fmt.Sprintf("<b>Found %d keywords</b> in table %s.<br>", keywordEntries, html.EscapeString(c.Keywords)))
	}

	data.Message += template.HTML("<b>Validation complete.</b><br><br>")

	var rewriteIDs bool
	for _, b := range db.allBoards() {
		if len(db.allThreads(b, false)) != 0 {
			rewriteIDs = true
			break
		}
	}
	if rewriteIDs {
		data.Message += template.HTML("Found existing Sriracha posts.<br><b>Post IDs will be rewritten.</b><br><br>")
	}

	doImport := formBool(r, "import")
	if !doImport {
		data.Message += template.HTML(`<form method="post"><input type="hidden" name="import" value="1">
        <table border="0" class="manageform">
            <tr>
                <td class="postblock"><label for="dir">Board Directory</label></td>
                <td><input type="text" name="dir"></td>
                <td>The directory where the board files are located. If the board is located at the server root, leave blank.</td>
            </tr>
            <tr>
                <td class="postblock"><label for="name">Board Name</label></td>
                <td><input type="text" name="name"></td>
                <td>The name of the board, which is displayed in the page title and header.</td>
            </tr>
            <tr>
                <td></td>
                <td><input type="submit" value="Start dry run"></td>
				<td></td>
            </tr>
		</table>
		</form>`)
		return
	}

	data.Message += template.HTML("Creating board...<br>")
	b := newBoard()
	b.Dir = formString(r, "dir")
	b.Name = formString(r, "name")
	err = b.validate()
	if err != nil {
		data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to validate board: %s", html.EscapeString(err.Error())))
		return
	}
	match := db.boardByDir(b.Dir)
	if match != nil {
		data.Message += template.HTML("<b>Error:</b> A board with that directory already exists in Sriracha.")
		return
	}
	db.addBoard(b)
	data.Message += template.HTML("<b>Board created.</b><br><br>")

	// Collect post IDs.
	data.Message += template.HTML("Collecting post IDs...<br>")
	rows, err := conn.Query(context.Background(), "SELECT id FROM "+c.Posts+" ORDER BY id ASC")
	if err != nil {
		data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to select posts in table %s: %s", html.EscapeString(c.Posts), err))
		return
	}
	var postIDs []int
	for rows.Next() {
		var postID int
		err := rows.Scan(&postID)
		if err != nil {
			data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to select posts in table %s: %s", html.EscapeString(c.Posts), err))
			return
		}
		postIDs = append(postIDs, postID)
	}
	data.Message += template.HTML("<b>Post IDs collected.</b><br><br>")

	data.Message += template.HTML("Verifying board directories...<br>")
	dirs := []string{b.Dir, filepath.Join(b.Dir, "src"), filepath.Join(b.Dir, "thumb"), filepath.Join(b.Dir, "res")}
	for _, dir := range dirs {
		dirPath := filepath.Join(s.config.Root, dir)
		_, err := os.Stat(dirPath)
		if os.IsNotExist(err) {
			data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Board directory %s does not exist.", html.EscapeString(dirPath)))
			return
		}
		if unix.Access(dirPath, unix.W_OK) != nil {
			data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Board directory %s is not writable.", html.EscapeString(dirPath)))
			return
		}
	}
	data.Message += template.HTML("<b>Board directories exist and are writable.</b><br><br>")

	type importPost struct {
		ID                int
		Parent            int
		Timestamp         int64
		Bumped            int64
		IP                string
		Name              string
		Tripcode          string
		Email             string
		NameBlock         string
		Subject           string
		Message           string
		Password          string
		File              string
		FileHash          string
		FileOriginal      string
		FileSize          int64
		FileSizeFormatted string
		FileWidth         int
		FileHeight        int
		Thumb             string
		ThumbWidth        int
		ThumbHeight       int
		Moderated         int
		Stickied          int
		Locked            int
	}

	data.Message += template.HTML("Importing posts...<br>")
	newIDs := make(map[int]int)
	var lastPostID int
	for _, postID := range postIDs {
		var p importPost
		err := conn.QueryRow(context.Background(), "SELECT * FROM "+c.Posts+" WHERE id = $1", postID).Scan(
			&p.ID,
			&p.Parent,
			&p.Timestamp,
			&p.Bumped,
			&p.IP,
			&p.Name,
			&p.Tripcode,
			&p.Email,
			&p.NameBlock,
			&p.Subject,
			&p.Message,
			&p.Password,
			&p.File,
			&p.FileHash,
			&p.FileOriginal,
			&p.FileSize,
			&p.FileSizeFormatted,
			&p.FileWidth,
			&p.FileHeight,
			&p.Thumb,
			&p.ThumbWidth,
			&p.ThumbHeight,
			&p.Moderated,
			&p.Stickied,
			&p.Locked)
		if err != nil {
			data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to select posts in table %s: %s", html.EscapeString(c.Posts), err))
			return
		}
		pp := &Post{
			ID:           p.ID,
			Board:        b,
			Parent:       p.Parent,
			Timestamp:    p.Timestamp,
			Bumped:       p.Bumped,
			IP:           "",
			Name:         p.Name,
			Tripcode:     p.Tripcode,
			Email:        p.Email,
			NameBlock:    p.NameBlock,
			Subject:      p.Subject,
			Message:      p.Message,
			Password:     "",
			File:         p.File,
			FileHash:     "",
			FileOriginal: "",
			FileSize:     p.FileSize,
			FileWidth:    p.FileWidth,
			FileHeight:   p.FileHeight,
			Thumb:        p.Thumb,
			ThumbWidth:   p.ThumbWidth,
			ThumbHeight:  p.ThumbHeight,
			Moderated:    PostModerated(p.Moderated),
			Stickied:     p.Stickied == 1,
			Locked:       p.Locked == 1,
		}
		hashLen := len(p.FileHash)
		isEmbed := hashLen != 0 && hashLen < 32
		if isEmbed {
			pp.FileHash = fmt.Sprintf("e %s %s", p.FileHash, p.FileOriginal)
		} else {
			pp.FileOriginal = p.FileOriginal
			if p.File != "" {
				srcPath := filepath.Join(s.config.Root, b.Dir, "src", p.File)
				buf, err := os.ReadFile(srcPath)
				if err != nil {
					data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> File not found at %s", html.EscapeString(srcPath)))
					return
				}
				checksum := sha512.Sum384(buf)
				pp.FileHash = base64.URLEncoding.EncodeToString(checksum[:])
				if p.Thumb != "" {
					thumbPath := filepath.Join(s.config.Root, b.Dir, "thumb", p.Thumb)
					_, err := os.Stat(thumbPath)
					if os.IsNotExist(err) {
						data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Thumbnail not found at %s", html.EscapeString(srcPath)))
						return
					}
				}
			}
		}

		carriageReturn := regexp.MustCompile(`(?s)\r.?`)
		pp.Message = carriageReturn.ReplaceAllStringFunc(pp.Message, func(s string) string {
			if len(s) == 1 || s[1] == '\n' {
				return "\n"
			}
			return "\n" + string(s[1])
		})

		resPattern := regexp.MustCompile(`<a href="res\/([0-9]+).html#([0-9]+)" class="([A-Aa-z]+)">&gt;&gt;([0-9]+)</a>`)
		pp.Message = resPattern.ReplaceAllStringFunc(pp.Message, func(s string) string {
			match := resPattern.FindStringSubmatch(s)
			threadID := parseInt(match[1])
			postID := parseInt(match[2])
			return fmt.Sprintf(`<a href="%sres/%d.html#%d" class="%s">&gt;&gt;%d</a>`, b.Path(), newIDs[threadID], newIDs[postID], match[3], newIDs[postID])
		})

		if pp.Parent != 0 {
			pp.Parent = newIDs[pp.Parent]
		}
		match := db.postByFileHash(pp.FileHash)
		if match != nil {
			pp.FileHash = ""
		}
		if rewriteIDs {
			db.addPost(pp)
		} else {
			var parent *int
			if pp.Parent != 0 {
				parent = &pp.Parent
			}
			var fileHash *string
			if pp.FileHash != "" {
				fileHash = &pp.FileHash
			}
			var stickied int
			if pp.Stickied {
				stickied = 1
			}
			var locked int
			if pp.Locked {
				locked = 1
			}
			err = db.conn.QueryRow(context.Background(), "INSERT INTO post VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25) RETURNING id",
				pp.ID,
				parent,
				pp.Board.ID,
				pp.Timestamp,
				pp.Bumped,
				pp.IP,
				pp.Name,
				pp.Tripcode,
				pp.Email,
				pp.NameBlock,
				pp.Subject,
				pp.Message,
				pp.Password,
				pp.File,
				fileHash,
				pp.FileOriginal,
				pp.FileSize,
				pp.FileWidth,
				pp.FileHeight,
				pp.Thumb,
				pp.ThumbWidth,
				pp.ThumbHeight,
				pp.Moderated,
				stickied,
				locked,
			).Scan(&pp.ID)
			if err != nil || pp.ID == 0 {
				data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to insert post: %s", err))
				return
			}
		}
		lastPostID = pp.ID
		newIDs[p.ID] = pp.ID
	}
	data.Message += template.HTML(fmt.Sprintf("<b>Imported %d posts.</b><br><br>", len(postIDs)))

	type importKeyword struct {
		ID     int
		Text   string
		Action string
	}

	if c.Keywords != "" {
		var imported int
		data.Message += template.HTML("Importing keywords...<br>")
		rows, err := conn.Query(context.Background(), "SELECT * FROM "+c.Keywords+" ORDER BY id ASC")
		if err != nil {
			data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to select keywords in table %s: %s", html.EscapeString(c.Keywords), err))
			return
		}
		for rows.Next() {
			k := &importKeyword{}
			err := rows.Scan(&k.ID, &k.Text, &k.Action)
			if err != nil {
				data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to select keywords in table %s: %s", html.EscapeString(c.Keywords), err))
				return
			}
			kk := &Keyword{
				Text:   k.Text,
				Action: k.Action,
				Boards: []*Board{b},
			}
			if strings.HasPrefix(kk.Text, "regexp:") {
				kk.Text = strings.TrimPrefix(kk.Text, "regexp:")
			} else {
				kk.Text = regexp.QuoteMeta(kk.Text)
			}
			err = kk.validate()
			if err != nil {
				data.Message += template.HTML(fmt.Sprintf("<b>Warning:</b> Skipping keyword #%d: %s<br>", k.ID, err))
				continue
			}
			match := db.keywordByText(kk.Text)
			if match != nil {
				data.Message += template.HTML(fmt.Sprintf("<b>Warning:</b> Skipping keyword #%d: keyword already exists in Sriracha<br>", k.ID))
				continue
			}
			db.addKeyword(kk)
			imported++
		}
		data.Message += template.HTML(fmt.Sprintf("<b>Imported %d keywords.</b><br><br>", imported))
	}

	if lastPostID != 0 {
		_, err := db.conn.Exec(context.Background(), "ALTER SEQUENCE post_id_seq RESTART WITH "+strconv.Itoa(lastPostID+1))
		if err != nil {
			data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to update post auto-increment value: %s", html.EscapeString(err.Error())))
			return
		}
	}

	if !commit {
		data.Message += template.HTML("<b>Dry run successful.</b><br>Ready to import.<br><br>")
		data.Message += template.HTML(`<form method="post">
		<input type="hidden" name="import" value="1">
		<input type="hidden" name="confirmation" value="1">
		<input type="hidden" name="dir" value="` + html.EscapeString(b.Dir) + `">
		<input type="hidden" name="name" value="` + html.EscapeString(b.Name) + `">
		<input type="submit" value="Start import">
		</form>`)
		return
	}

	s.config.importComplete = true
	s.rebuildBoard(db, b)
}
