package sriracha

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Server) serveImport(data *templateData, db *Database, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleSuperAdmin) {
		return
	} else if !s.config.importMode {
		data.ManageError("Sriracha is not running in import mode.")
		return
	}
	c := s.config.Import

	var commit bool
	defer func() {
		var command = "ROLLBACK"
		if commit {
			command = "COMMIT"
		}
		db.conn.Exec(context.Background(), command)
	}()

	data.Template = "manage_info"
	data.Message = `<h2 class="managetitle">Import</h2><b>Warning:</b> Backup all files and databases before importing a board.<br><br>`

	// Connect to the database.
	data.Message += template.HTML("Connecting to database...<br>")
	databaseURL := fmt.Sprintf("postgres://%s:%s@%s/%s", c.Username, c.Password, c.Address, c.DBName)
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		data.Message += template.HTML("<b>Error:</b> Failed to connect to database: " + html.EscapeString(err.Error()))
		return
	}
	data.Message += template.HTML("<b>Connected.</b><br><br>")

	// Start transaction.
	data.Message += template.HTML("Starting database transaction...<br>")
	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		data.Message += template.HTML("<b>Error:</b> Failed to verify connection status: " + html.EscapeString(err.Error()))
		return
	}
	data.Message += template.HTML("<b>Transaction started.</b><br><br>")

	// Validate posts table.
	data.Message += template.HTML("Validating posts table...<br>")
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
	data.Message += template.HTML(fmt.Sprintf("<b>Found %d posts</b> in table %s.<br><br>", postEntries, html.EscapeString(c.Posts)))

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
	data.Message += template.HTML("<b>IDs collected.</b>.<br><br>")

	// TODO
	data.Message += template.HTML("Creating board...<br>")
	b := &Board{
		Dir:  fmt.Sprintf("asdajsf%d", time.Now().Unix()),
		Name: "ajsdhasd",
	}
	db.addBoard(b)
	data.Message += template.HTML("<b>Board created.</b>.<br><br>")

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
	for _, postID := range postIDs {
		rows, err = conn.Query(context.Background(), "SELECT * FROM "+c.Posts+" WHERE id = $1", postID)
		if err != nil {
			data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to select posts in table %s: %s", html.EscapeString(c.Posts), err))
			return
		}
		for rows.Next() {
			var p importPost
			err := rows.Scan(&p.ID,
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
				Stickied:     p.Stickied,
				Locked:       p.Locked,
			}
			hashLen := len(p.FileHash)
			isEmbed := hashLen != 0 && hashLen < 32
			if isEmbed {
				pp.FileHash = fmt.Sprintf("e %s %s", p.FileHash, p.FileOriginal)
			} else {
				// TODO Set updated hash.
				pp.FileOriginal = p.FileOriginal
			}

			var parent *int
			if pp.Parent != 0 {
				parent = &pp.Parent
			}
			var fileHash *string
			if pp.FileHash != "" {
				fileHash = &pp.FileHash
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
				pp.Stickied,
				pp.Locked,
			).Scan(&pp.ID)
			if err != nil || pp.ID == 0 {
				data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> Failed to insert post: %s", err))
				return
			}

			log.Printf("%+v", pp)
		}
	}
	data.Message += template.HTML("<b>Imported posts.</b.<br><br>")
}
