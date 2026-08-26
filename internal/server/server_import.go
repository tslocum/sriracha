package server

import (
	"archive/zip"
	"database/sql"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/gabriel-vasile/mimetype"

	_ "github.com/go-sql-driver/mysql"
)

var ytEmbedPattern = regexp.MustCompile(`\/\/www\.youtube\.com\/embed\/([0-9A-Za-z_\-]+)`)

// importHandler describes the required methods for handling importing posts from an external database.
type importHandler interface {
	// Name returns the name of the software supported by the handler.
	Name() string

	// Matches returns whether the handler recognizes the database being imported.
	Matches() bool

	// Tables returns a list of post tables.
	Tables() []string

	// Posts returns the posts contained in the specified table.
	Posts(table string) []*Post
}

type importInfo struct {
	name    string
	sqlDB   *sql.DB
	handler importHandler
	table   string
	posts   []*Post
}

func (s *Server) importHandlers(db *sql.DB) []importHandler {
	return []importHandler{
		&vichanImport{db: db},
		&srirachaImport{db: db},
		&tinyibImport{db: db},
	}
}

func (s *Server) _importExternal(importPath string) error {
	var db *sql.DB
	var err error
	switch {
	case strings.HasPrefix(importPath, "mysql://"):
		db, err = sql.Open("mysql", importPath[8:])
		if err != nil {
			return fmt.Errorf("failed to connect to database: %s", err)
		}
	}
	if db == nil {
		return fmt.Errorf("unrecognized database type: expected 'mysql://...'")
	}

	var handlers []importHandler
	allHandlers := s.importHandlers(db)
	for _, handler := range allHandlers {
		if handler.Matches() {
			handlers = append(handlers, handler)
		}
	}
	if len(handlers) == 0 {
		return fmt.Errorf("no import handlers recognize the provided database (see manual for list of supported software)")
	} else if len(handlers) > 1 {
		var names []string
		for _, handler := range handlers {
			names = append(names, handler.Name())
		}
		return fmt.Errorf("multiple import handlers claim to match the provided database: %+v", names)
	}

	tables := handlers[0].Tables()
	if len(tables) == 0 {
		log.Fatalf("no tables containing posts were found")
	}
	for _, table := range tables {
		if len(handlers[0].Posts(table)) == 0 {
			continue
		}
		info := &importInfo{
			name:    fmt.Sprintf("%s (%s)", handlers[0].Name(), table),
			sqlDB:   db,
			handler: handlers[0],
			table:   table,
		}
		s.importDatabases = append(s.importDatabases, info)
	}
	return nil
}

func (s *Server) _importDatabase(name string, filePath string) error {
	sqlDB, err := sql.Open("sqlite", filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: expected SQLite database file", filePath)
	}
	var handler importHandler
	srirachaHandler := &srirachaImport{db: sqlDB}
	if srirachaHandler.Matches() {
		handler = srirachaHandler
	} else {
		tinyibHandler := &tinyibImport{db: sqlDB}
		if tinyibHandler.Matches() {
			handler = tinyibHandler
		} else {
			return fmt.Errorf("unrecognized database")
		}
	}
	for _, table := range handler.Tables() {
		if len(handler.Posts(table)) == 0 {
			continue
		}
		info := &importInfo{
			name:    name,
			sqlDB:   sqlDB,
			handler: handler,
			table:   table,
		}
		s.importDatabases = append(s.importDatabases, info)
	}
	return nil
}

func (s *Server) importDatabase(importPath string) error {
	if strings.Contains(importPath, "://") {
		return s._importExternal(importPath)
	}

	_, err := os.Stat(importPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("file %s does not exist", importPath)
	}
	z, err := zip.OpenReader(importPath)
	if err != nil {
		return s._importDatabase(filepath.Base(importPath), importPath)
	}
	for _, f := range z.File {
		tmpFile, err := os.CreateTemp("", "*.db")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %s", err)
		}
		zf, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open file %s in archive: %s", f.Name, err)
		}
		_, err = io.Copy(tmpFile, zf)
		if err != nil {
			return fmt.Errorf("failed to extract file %s from archive: %s", f.Name, err)
		}
		err = s._importDatabase(f.Name, tmpFile.Name())
		if err != nil {
			return fmt.Errorf("failed to import file %s: %s", f.Name, err)
		}
	}
	return nil
}

func (s *Server) _importPost(p *Post, tinyIB bool) error {
	if p.Board == nil {
		return nil
	}

	// Import TinyIB embed attachments.
	if tinyIB && (p.FileHash == "YouTube" || p.FileHash == "Vimeo" || p.FileHash == "SoundCloud") {
		ytVideo := p.FileHash == "YouTube"

		// Fix file hash.
		p.FileHash = "e " + p.FileHash + " " + p.FileOriginal
		p.FileOriginal = ""

		// Extract video URL from embed HTML.
		if ytVideo {
			m := ytEmbedPattern.FindStringSubmatch(p.File)
			if len(m) > 1 {
				p.FileOriginal = "https://www.youtube.com/watch?v=" + m[1]
			}
		}
	}

	// Fill bumped.
	if p.Bumped <= 0 {
		p.Bumped = p.Timestamp
	}

	// Fill nameblock.
	if p.NameBlock == "" {
		p.SetNameBlock(p.Board.DefaultName, "", false, false)
	}

	if p.File != "" && !p.IsEmbed() {
		srcPath := filepath.Join(s.config.Root, p.Board.Dir, "src", p.File)

		srcFile, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open attachment %s of post No.%d: %s", p.File, p.ID, err)
		}
		defer srcFile.Close()

		// Fill filemime.
		if p.FileMIME == "" {
			mime, err := mimetype.DetectReader(srcFile)
			if err == nil {
				p.FileMIME = mime.String()
			}
		}

		// Rebuild filehash and filesize.
		hash := s.newHash()
		srcFile.Seek(0, 0)
		fileSize, err := io.Copy(hash, srcFile)
		if err != nil {
			return fmt.Errorf("failed to calculate hash of attachment %s of post No.%d: %s", p.File, p.ID, err)
		}
		var sum [HashSize]byte
		hash.Sum(sum[:0])
		p.FileHash = base64.URLEncoding.EncodeToString(sum[:])
		p.FileSize = fileSize

		// Fill filewidth and fileheight.
		if p.FileWidth <= 0 || p.FileHeight <= 0 {
			isImage := p.FileMIME == "image/jpeg" || p.FileMIME == "image/pjpeg" || p.FileMIME == "image/png" || p.FileMIME == "image/gif"
			if isImage {
				srcFile.Seek(0, 0)
				imgWidth, imgHeight := s.imageDimensions(srcFile)
				if imgWidth == 0 || imgHeight == 0 {
					return fmt.Errorf("failed to calculate width and height of attachment %s of post No.%d: %s", p.File, p.ID, err)
				}
				p.FileWidth, p.FileHeight = imgWidth, imgHeight
			}
		}
	}
	if p.Thumb != "" {
		thumbPath := filepath.Join(s.config.Root, p.Board.Dir, "thumb", p.Thumb)

		thumbFile, err := os.Open(thumbPath)
		if err != nil {
			return fmt.Errorf("failed to open attachment %s of post No.%d: %s", p.File, p.ID, err)
		}
		defer thumbFile.Close()

		// Fill thumbwidth and thumbheight.
		if p.ThumbWidth <= 0 || p.ThumbHeight <= 0 {
			mime, err := mimetype.DetectReader(thumbFile)
			if err == nil {
				thumbFile.Seek(0, 0)
				mimeType := mime.String()
				if mimeType == "image/jpeg" || mimeType == "image/pjpeg" || mimeType == "image/png" || mimeType == "image/gif" {
					imgWidth, imgHeight := s.imageDimensions(thumbFile)
					if imgWidth != 0 && imgHeight != 0 {
						p.ThumbWidth, p.ThumbHeight = imgWidth, imgHeight
					}
				}
			}
			if p.ThumbWidth <= 0 || p.ThumbHeight <= 0 {
				p.Thumb = ""
				p.ThumbWidth = 0
				p.ThumbHeight = 0
			}
		}
	}
	return nil
}

func (s *Server) importPosts(info *importInfo, b *Board) ([]*Post, error) {
	tinyIB := info.handler.Name() == "TinyIB"
	posts := info.handler.Posts(info.table)
	for _, p := range posts {
		if b == nil {
			continue
		}
		p.Board = b
		err := s._importPost(p, tinyIB)
		if err != nil {
			return nil, err
		}
	}
	return posts, nil
}

func (s *Server) serveImport(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_info"
	data.Boards = db.AllBoards()
	data.Message = `<h2 class="managetitle">` + GetHTML(nil, data.Account, "Import") + `</h2>`

	completeMessage := "<b>" + Get(nil, data.Account, "Import complete.") + "</b><br>" + Get(nil, data.Account, "Please restart Sriracha without any import options.") + "<br>"
	if data.forbidden(w, RoleSuperAdmin) {
		return
	} else if !s.config.ImportMode {
		data.ManageError(Get(nil, data.Account, "Sriracha is not running in import mode."))
		return
	} else if s.config.ImportComplete {
		data.Message += template.HTML(completeMessage)
		return
	}

	data.Template = "manage_info"
	data.Message += `<b>` + GetHTML(nil, data.Account, "Warning") + `:</b> ` + GetHTML(nil, data.Account, "Backup all files and databases before importing posts.") + `<br><br>`

	commit := FormBool(r, "import") && FormBool(r, "confirm")
	defer func() {
		if commit && s.config.ImportComplete {
			err := db.CommitWithErr()
			if err != nil {
				data.ManageError("Failed to commit changes: " + err.Error())
				return
			} else {
				data.Message += template.HTML(completeMessage)
			}
		} else {
			db.RollBack()
		}
	}()

	var haveMapping bool
	importBoards := make([]*Board, len(s.importDatabases))
	for i := range s.importDatabases {
		boardID := FormInt(r, fmt.Sprintf("board%d", i))
		if boardID > 0 {
			importBoards[i] = db.BoardByID(boardID)
			if importBoards[i] != nil {
				haveMapping = true
			}
		}
	}

	// Validate table.
	for i, info := range s.importDatabases {
		posts, err := s.importPosts(info, importBoards[i])
		if err != nil {
			data.ManageError(fmt.Sprintf("Failed to load export %s: %s", info.name, err.Error()))
			return
		} else if len(posts) == 0 {
			data.ManageError(fmt.Sprintf("No posts were found in export %s.", info.name))
			return
		}
		s.importDatabases[i].posts = posts
	}

	if !haveMapping {
		data.Message += `<table class="managetable"><tbody>
		<tr>
			<th>` + GetHTML(nil, data.Account, "Export") + `</th>
			<th>` + GetHTML(nil, data.Account, "Posts") + `</th>
			<th>` + GetHTML(nil, data.Account, "Threads") + `</th>
			<th>` + GetHTML(nil, data.Account, "Replies") + `</th>
			<th>` + GetHTML(nil, data.Account, "Attachments") + `</th>
		</tr>`
		for _, info := range s.importDatabases {
			name := strings.TrimSuffix(strings.TrimSuffix(info.name, ".db"), ".sriracha")
			var threads, replies, attachments int
			for _, p := range info.posts {
				if p.Parent == 0 {
					threads++
				} else {
					replies++
				}
				if p.File != "" {
					attachments++
				}
			}
			data.Message += template.HTML(s.msgPrinter.Sprintf("<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td></tr>", html.EscapeString(name), threads+replies, threads, replies, attachments))
		}
		data.Message += `</tbody></table>`
	} else {
		for i, info := range s.importDatabases {
			if haveMapping && importBoards[i] == nil {
				continue
			}
			for _, p := range info.posts {
				if strings.HasPrefix(p.FileHash, "e ") && p.File == "" {
					err := s.embedMedia(db, p, p.FileOriginal, !commit)
					if err != nil {
						data.ManageError(err.Error())
						return
					}
				}
			}
		}
	}

	if !haveMapping || !commit {
		if !haveMapping {
			data.Message += template.HTML("<br><b>" + Get(nil, data.Account, "Export files loaded.") + "</b><br>" + Get(nil, data.Account, "Ready to start dry run.") + "<br>")
		} else if !commit {
			data.Message += template.HTML("<b>" + Get(nil, data.Account, "Dry run complete.") + "</b><br>" + Get(nil, data.Account, "Ready to import posts.") + "<br>")
		}

		data.Message += template.HTML(`<br><fieldset>
		<legend>` + Get(nil, data.Account, "Boards") + `</legend>
		<form method="post">
		<input type="hidden" name="import" value="1">`)
		if !haveMapping {
			data.Message += template.HTML(Get(nil, data.Account, "Choose where to import posts") + `:<br><br>`)
		} else {
			data.Message += template.HTML(`<input type="hidden" name="confirm" value="1">`)
		}
		data.Message += template.HTML(`<table class="manageform">`)
		var disabled string
		if haveMapping {
			disabled = " disabled"
		}
		var selected string
		for i, info := range s.importDatabases {
			if haveMapping && importBoards[i] == nil {
				continue
			}
			name := strings.TrimSuffix(strings.TrimSuffix(info.name, ".db"), ".sriracha")
			data.Message += template.HTML(fmt.Sprintf(`<tr>
				<td class="postblock"><label for="board%d">%s</label></td>
				<td><select name="board%d"%s>
					<option value="0">`+Get(nil, data.Account, "Do not import")+`</option>`, i, name, i, disabled))
			for _, b := range data.Boards {
				label := b.Path()
				if b.Name != "" {
					label += " " + b.Name
				}
				selected = ""
				if importBoards[i] != nil && b.ID == importBoards[i].ID {
					selected = " selected"
				}
				data.Message += template.HTML(fmt.Sprintf(`<option value="%d"%s>%s</option>`, b.ID, selected, label))
			}
			data.Message += template.HTML(`</select></td>
			</tr>`)
		}
		data.Message += template.HTML(`
				<tr>
					<td style="vertical-align: middle;">&nbsp;`)
		if !haveMapping {
			data.Message += template.HTML(`[<a href="/sriracha/board/">` + Get(nil, data.Account, "Manage Boards") + `</a>]`)
		}
		label := "Start Dry Run"
		if haveMapping {
			label = "Start Import"
		}
		data.Message += template.HTML(`</td>
                    <td><input type="submit" value="` + G(nil, data.Account, label) + `"></td>
					<td></td>
				</tr>
			</table>`)
		if haveMapping {
			for i := range s.importDatabases {
				b := importBoards[i]
				if b != nil {
					data.Message += template.HTML(fmt.Sprintf(`<input type="hidden" name="board%d" value="%d">`, i, b.ID))
				}
			}
		}
		data.Message += template.HTML(`</form>
		</fieldset><br>`)
		return
	}

	var lastPostID int
	for i, info := range s.importDatabases {
		b := importBoards[i]
		if b == nil {
			continue
		}

		var rewriteIDs bool
		for _, p := range info.posts {
			if p.ID <= 0 {
				data.ManageError(fmt.Sprintf("Invalid post: no post ID: %+v", *p))
				return
			}
			dbPost := db.PostByID(p.ID)
			if dbPost != nil {
				rewriteIDs = true
				break
			}
		}

		newIDs := make(map[int]int)
		for _, p := range info.posts {
			carriageReturn := regexp.MustCompile(`(?s)\r.?`)
			p.Message = carriageReturn.ReplaceAllStringFunc(p.Message, func(s string) string {
				if len(s) == 1 || s[1] == '\n' {
					return "\n"
				}
				return "\n" + string(s[1])
			})

			resPattern := regexp.MustCompile(`<a href="[^"]*res\/([0-9]+).html#([0-9]+)" class="([A-Aa-z]+)">&gt;&gt;([0-9]+)(\(OP\))?</a>`)
			p.Message = resPattern.ReplaceAllStringFunc(p.Message, func(s string) string {
				match := resPattern.FindStringSubmatch(s)
				threadID := ParseInt(match[1])
				postID := ParseInt(match[2])
				if newIDs[threadID] == 0 || newIDs[postID] == 0 {
					return s
				}
				var extra string
				if postID == threadID {
					extra = "(OP)"
				}
				return fmt.Sprintf(`<a href="%sres/%d.html#%d" class="%s">&gt;&gt;%d%s</a>`, b.Path(), newIDs[threadID], newIDs[postID], match[3], newIDs[postID], extra)
			})

			p.Message = strings.TrimSuffix(p.Message, "<br>")

			if p.Parent != 0 {
				p.Parent = newIDs[p.Parent]
			}
			oldID := p.ID
			if rewriteIDs {
				db.AddPost(p)
			} else {
				var parent *int
				if p.Parent != 0 {
					parent = &p.Parent
				}
				var fileHash *string
				if p.FileHash != "" {
					fileHash = &p.FileHash
				}
				var stickied int
				if p.Stickied {
					stickied = 1
				}
				var locked int
				if p.Locked {
					locked = 1
				}
				_, err := db.Exec("INSERT INTO post VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, DEFAULT, to_tsvector($27))",
					p.ID,
					parent,
					p.Board.ID,
					p.Timestamp,
					p.Bumped,
					p.IP,
					p.Name,
					p.Tripcode,
					p.Email,
					p.NameBlock,
					p.Subject,
					p.Message,
					p.Password,
					p.File,
					fileHash,
					p.FileOriginal,
					p.FileSize,
					p.FileWidth,
					p.FileHeight,
					p.Thumb,
					p.ThumbWidth,
					p.ThumbHeight,
					p.Moderated,
					stickied,
					locked,
					p.FileMIME,
					p.SearchText(),
				)
				if err != nil {
					data.ManageError(fmt.Sprintf("Failed to insert post: %s.", err))
					return
				}
			}
			db.AddPostBacklinks(p)
			newIDs[oldID] = p.ID
			if p.ID > lastPostID {
				lastPostID = p.ID
			}
		}

		if rewriteIDs {
			_, err := db.Exec("ALTER SEQUENCE post_id_seq RESTART WITH " + strconv.Itoa(db.MaxPostID()+1))
			if err != nil {
				data.ManageError(fmt.Sprintf("Failed to update post auto-increment value: %s.", err))
				return
			}
		}
	}

	s.config.ImportComplete = true
	db.SoftCommit()
	s.rebuildAll(db)
}
