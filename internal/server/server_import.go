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

	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/gabriel-vasile/mimetype"
)

var ytEmbedPattern = regexp.MustCompile(`\/\/www\.youtube\.com\/embed\/([0-9A-Za-z_\-]+)`)

type importInfo struct {
	name  string
	sqlDB *sql.DB
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
					if imgWidth == 0 || imgHeight == 0 {
						return fmt.Errorf("failed to calculate width and height of attachment %s of post No.%d: %s", p.File, p.ID, err)
					}
					p.ThumbWidth, p.ThumbHeight = imgWidth, imgHeight
				}
			}
		}
	}
	return nil
}

func (s *Server) importPosts(sqlDB *sql.DB, table string, tinyIB bool, b *Board, commit bool) (int, error) {
	// Build query.
	var query string
	if tinyIB {
		query = "SELECT id, parent, timestamp, bumped, name, tripcode, email, nameblock, subject, message, file, '' AS file_mime, file_hex, file_original, file_size, image_width, image_height, thumb, thumb_width, thumb_height, stickied, locked FROM " + table
	} else {
		query = "SELECT * FROM " + table
	}
	query += " ORDER BY id ASC"

	// Query database for posts.
	var posts int
	rows, err := sqlDB.Query(query)
	if err != nil {
		return 0, err
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
			return 0, err
		}
		posts++

		if b == nil {
			continue
		}
		p.Board = b
		err = s._importPost(p, tinyIB)
		if err != nil {
			return 0, err
		}
	}
	return posts, nil
}

func (s *Server) serveImport(data *templateData, db *database.DB, w http.ResponseWriter, r *http.Request) {
	data.Template = "manage_info"
	data.Boards = db.AllBoards()
	data.Message = `<h2 class="managetitle">Import</h2>`

	const completeMessage = "<b>Import complete.</b><br>Please restart Sriracha without the --import flag.<br>"
	if data.forbidden(w, RoleSuperAdmin) {
		return
	} else if !s.config.ImportMode {
		data.ManageError("Sriracha is not running in import mode.")
		return
	} else if s.config.ImportComplete {
		data.Message += template.HTML(completeMessage)
		return
	}

	data.Template = "manage_info"
	data.Message += `<b>Warning:</b> Backup all files and databases before importing posts.<br><br>`

	commit := FormBool(r, "import") && FormBool(r, "confirm")
	defer func() {
		if commit && s.config.ImportComplete {
			err := db.CommitWithErr()
			if err != nil {
				data.ManageError("Failed to commit changes: " + err.Error())
				return
			} else {
				data.Message += template.HTML("<br>" + completeMessage)
			}
		} else {
			db.RollBack()
		}
	}()

	var haveMapping bool
	importBoards := make([]*Board, len(s.importDatabases))

	// Validate table.
	for i, info := range s.importDatabases {
		sqlDB := info.sqlDB

		boardID := FormInt(r, fmt.Sprintf("board%d", i))
		if boardID > 0 {
			importBoards[i] = db.BoardByID(boardID)
			if importBoards[i] != nil {
				haveMapping = true
			}
		}

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
			if table != "" {
				tinyIB = i == 1
				break
			}
		}

		if table == "" {
			data.ManageError(fmt.Sprintf("Failed to locate post table in export %s", info.name))
			return
		}
		postCount, err := s.importPosts(sqlDB, table, tinyIB, importBoards[i], false)
		if err != nil {
			data.ManageError(fmt.Sprintf("Failed to query post table in export %s: %s", info.name, err.Error()))
			return
		} else if postCount == 0 {
			data.ManageError(fmt.Sprintf("No posts were found in export %s.", info.name))
			return
		}
		data.Message += template.HTML(fmt.Sprintf("<b>Found %d posts</b> in export %s.<br>", postCount, html.EscapeString(info.name)))
	}

	if !haveMapping {
		data.Message += template.HTML("<br><b>Export files loaded.</b><br>Ready to start dry run.<br>")
	} else if !commit {
		data.Message += template.HTML("<br><b>Dry run successful.</b><br>Ready to import posts.<br>")
	} else {
		s.config.ImportComplete = true
		s.rebuildAll(db, false)
		return
	}

	data.Message += template.HTML(`<br><fieldset>
		<legend>Boards</legend>
		<form method="post">
		<input type="hidden" name="import" value="1">`)
	if !haveMapping {
		data.Message += template.HTML(`Choose where to import posts:<br><br>`)
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
		data.Message += template.HTML(fmt.Sprintf(`<tr>
				<td class="postblock"><label for="board%d">%s</label></td>
				<td><select name="board%d"%s>
					<option value="0">Do not import</option>`, i, info.name, i, disabled))
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
		data.Message += template.HTML(`[<a href="/sriracha/board/">Manage Boards</a>]`)
	}
	label := "Start Dry Run"
	if haveMapping {
		label = "Start Import"
	}
	data.Message += template.HTML(`</td>
                    <td><input type="submit" value="` + label + `"></td>
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
	/*
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

					pp.FileMIME = mimetype.Detect(buf).String()

					pp.FileHash = s.hashBytes(buf, "")

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
				threadID := ParseInt(match[1])
				postID := ParseInt(match[2])
				return fmt.Sprintf(`<a href="%sres/%d.html#%d" class="%s">&gt;&gt;%d</a>`, b.Path(), newIDs[threadID], newIDs[postID], match[3], newIDs[postID])
			})

			if pp.Parent != 0 {
				pp.Parent = newIDs[pp.Parent]
			}
			if rewriteIDs {
				db.AddPost(pp)
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
				err = db.QueryRow("INSERT INTO post VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26) RETURNING id",
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
					pp.FileMIME,
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

		if lastPostID != 0 {
			_, err := db.Exec("ALTER SEQUENCE post_id_seq RESTART WITH " + strconv.Itoa(lastPostID+1))
			if err != nil {
				data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to update post auto-increment value: %s", html.EscapeString(err.Error())))
				return
			}
		}
	*/
}
