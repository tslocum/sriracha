package server

import (
	"archive/zip"
	"database/sql"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/gabriel-vasile/mimetype"
)

var ytEmbedPattern = regexp.MustCompile(`\/\/www\.youtube\.com\/embed\/([0-9A-Za-z_\-]+)`)

type importInfo struct {
	name  string
	sqlDB *sql.DB
	posts []*Post
}

func (s *Server) _importDatabase(name string, filePath string) error {
	sqlDB, err := sql.Open("sqlite", filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: expected SQLite database file", filePath)
	}
	info := &importInfo{
		name:  name,
		sqlDB: sqlDB,
	}
	s.importDatabases = append(s.importDatabases, info)
	return nil
}

func (s *Server) importDatabase(importPath string) error {
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
		p.SetNameBlock(p.Board.DefaultName, "", false)
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

func (s *Server) importPosts(sqlDB *sql.DB, table string, tinyIB bool, b *Board) ([]*Post, error) {
	// Build query.
	var query string
	if tinyIB {
		// Import from TinyIB database.
		query = "SELECT id, parent, timestamp, bumped, name, tripcode, email, nameblock, subject, message, file, '' AS file_mime, file_hex, file_original, file_size, image_width, image_height, thumb, thumb_width, thumb_height, stickied, locked FROM " + table
	} else {
		// Import from Sriracha-compatible export.
		query = "SELECT id, parent, timestamp, bumped, name, tripcode, email, nameblock, subject, message, file, filemime, filehash, fileoriginal, filesize, filewidth, fileheight, thumb, thumbwidth, thumbheight, stickied, locked FROM " + table
	}
	query += " ORDER BY id ASC"

	// Query database for posts.
	var posts []*Post
	rows, err := sqlDB.Query(query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		p := &Post{}
		var stickied, locked int
		err = rows.Scan(&p.ID,
			&p.Parent,
			&p.Timestamp,
			&p.Bumped,
			&p.Name,
			&p.Tripcode,
			&p.Email,
			&p.NameBlock,
			&p.Subject,
			&p.Message,
			&p.File,
			&p.FileMIME,
			&p.FileHash,
			&p.FileOriginal,
			&p.FileSize,
			&p.FileWidth,
			&p.FileHeight,
			&p.Thumb,
			&p.ThumbWidth,
			&p.ThumbHeight,
			&stickied,
			&locked)
		if err != nil {
			return nil, err
		}
		p.Moderated = ModeratedVisible
		p.Stickied = stickied == 1
		p.Locked = locked == 1

		posts = append(posts, p)

		if b == nil {
			continue
		}
		p.Board = b
		err = s._importPost(p, tinyIB)
		if err != nil {
			return nil, err
		}
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return posts, nil
}

func (s *Server) serveImport(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_info"
	data.Boards = db.AllBoards()
	data.Message = `<h2 class="managetitle">` + GetHTML(nil, data.Account, "Import") + `</h2>`

	completeMessage := "<b>" + Get(nil, data.Account, "Import complete.") + "</b><br>" + Get(nil, data.Account, "Please restart Sriracha without the %s flag.", "--import") + "<br>"
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
		sqlDB := info.sqlDB

		// Locate post table.
		var table string
		var tinyIB bool
		for i := 0; i < 2; i++ {
			column := "filesize"
			if i == 1 {
				column = "file_size"
			}
			rows, err := sqlDB.Query("SELECT DISTINCT name FROM sqlite_master WHERE sql LIKE '%" + column + "%'")
			if err != nil {
				data.ManageError(err.Error())
				return
			}
			for rows.Next() {
				err = rows.Scan(&table)
				if err != nil {
					data.ManageError(err.Error())
					return
				}
			}
			if rows.Err() != nil {
				data.ManageError(rows.Err().Error())
				return
			}
			if table != "" {
				tinyIB = i == 1
				break
			}
		}
		if table == "" {
			data.ManageError(fmt.Sprintf("Failed to locate post table in export %s", info.name))
			return
		}

		posts, err := s.importPosts(sqlDB, table, tinyIB, importBoards[i])
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
			<th>` + GetHTML(nil, data.Account, "Threads") + `</th>
			<th>` + GetHTML(nil, data.Account, "Replies") + `</th>
			<th>` + GetHTML(nil, data.Account, "Posts") + `</th>
		</tr>`
		for _, info := range s.importDatabases {
			name := strings.TrimSuffix(strings.TrimSuffix(info.name, ".db"), ".sriracha")
			var threads, replies int
			for _, p := range info.posts {
				if p.Parent == 0 {
					threads++
				} else {
					replies++
				}
			}
			data.Message += template.HTML(s.msgPrinter.Sprintf("<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td></tr>", html.EscapeString(name), threads, replies, threads+replies))
		}
		data.Message += `</tbody></table>`
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

			resPattern := regexp.MustCompile(`<a href="[^"]*res\/([0-9]+).html#([0-9]+)" class="([A-Aa-z]+)">&gt;&gt;([0-9]+)</a>`)
			p.Message = resPattern.ReplaceAllStringFunc(p.Message, func(s string) string {
				match := resPattern.FindStringSubmatch(s)
				threadID := ParseInt(match[1])
				postID := ParseInt(match[2])
				if newIDs[threadID] == 0 || newIDs[postID] == 0 {
					return s
				}
				return fmt.Sprintf(`<a href="%sres/%d.html#%d" class="%s">&gt;&gt;%d</a>`, b.Path(), newIDs[threadID], newIDs[postID], match[3], newIDs[postID])
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
				_, err := db.Exec("INSERT INTO post VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)",
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
	s.rebuildAll(db, false)
}
