package sriracha

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"

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
	data.Message += template.HTML("Connected.<br><br>")

	// Start transaction.
	data.Message += template.HTML("Starting database transaction...<br>")
	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		data.Message += template.HTML("<b>Error:</b> Failed to verify connection status: " + html.EscapeString(err.Error()))
		return
	}
	data.Message += template.HTML("Transaction started.<br><br>")

	// Validate tables.
	data.Message += template.HTML("Validating tables...<br>")
	tableEntries := func(name string) (int, error) {
		var entries int
		err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM "+name).Scan(&entries)
		if err == pgx.ErrNoRows {
			return 0, nil
		} else if err != nil {
			return 0, fmt.Errorf("failed to select from table %s: %s", name, err)
		}
		return entries, nil
	}
	postEntries, err := tableEntries(c.Posts)
	if err != nil {
		data.Message += template.HTML("<b>Error:</b> Failed to validate tables: " + html.EscapeString(err.Error()))
		return
	} else if postEntries == 0 {
		data.Message += template.HTML(fmt.Sprintf("<b>Error:</b> No posts were found in table %s.", html.EscapeString(c.Posts)))
		return
	}
	data.Message += template.HTML(fmt.Sprintf("Found <b>%d</b> posts in table %s.", postEntries, html.EscapeString(c.Posts)))
}
