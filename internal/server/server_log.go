package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/jackc/pgx/v5/pgxpool"
)

const logPageSize = 25

func (s *Server) _logAudit(l *model.Log, conn *pgxpool.Conn) {
	var (
		accountID *int
		username  *string
		boardID   *int
	)
	if l.Account != nil {
		accountID = &l.Account.ID
		username = &l.Account.Username
	}
	if l.Board != nil {
		boardID = &l.Board.ID
	}
	timestamp := l.Timestamp
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	_, err := conn.Exec(context.Background(), "INSERT INTO sriracha_log VALUES (DEFAULT, $1, $2, $3, $4, $5, $6)", accountID, username, boardID, timestamp, l.Message, l.Changes)
	if err != nil {
		log.Fatalf("failed to insert audit log entry: %s", err)
	}
}

func (s *Server) logAudit(l *model.Log) {
	conn, err := s.auditPool.Acquire(context.Background())
	if err != nil {
		log.Fatalf("failed to acquire audit conn: %s", err)
	}
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		log.Fatalf("failed to begin audit transaction: %s", err)
	}

	s._logAudit(l, conn)

	_, err = conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		log.Fatalf("failed to commit audit transaction: %s", err)
	}
}

func (s *Server) connectAudit() error {
	config, err := pgxpool.ParseConfig(s.config.Audit)
	if err != nil {
		return fmt.Errorf("failed to parse database configuration: %w", err)
	}
	config.MinConns = 1
	config.MinIdleConns = 1
	config.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	s.auditPool = pool

	conn, err := pool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("failed to acquire audit conn: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		return fmt.Errorf("failed to begin audit transaction: %w", err)
	}

	var tableCount int
	err = conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'sriracha_log'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("failed to determine existing table count: %w", err)
	} else if tableCount == 0 {
		fmt.Println("Initializing audit database...")
		_, err = conn.Exec(context.Background(), `CREATE TABLE sriracha_log (
	id serial PRIMARY KEY,
	account smallint NULL,
	username text NULL,
	board smallint NULL,
	timestamp bigint NOT NULL,
	message text NOT NULL,
	changes text NOT NULL
)`)
		if err != nil {
			return fmt.Errorf("failed to create log table: %w", err)
		}
	} else if tableCount > 1 {
		return fmt.Errorf("found too many tables in audit database: expected 1, got %d", tableCount)
	}

	var auditCount int
	err = conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM sriracha_log").Scan(&auditCount)
	if err != nil {
		return fmt.Errorf("failed to determine existing log count: %w", err)
	}

	if auditCount == 0 {
		db := s.begin()
		dbCount := db.LogCount()
		if dbCount > 0 {
			fmt.Printf("Copying %d log entries to audit database...\n", dbCount)
			var page int
			for {
				logs := db.LogsByPage(page)
				if len(logs) == 0 {
					break
				}
				for i := range logs {
					s._logAudit(logs[i], conn)
				}
				page++
			}
		}
		db.Commit()
	}

	_, err = conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		return fmt.Errorf("failed to commit audit transaction: %w", err)
	}
	return nil
}

func (s *Server) serveLog(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	page := PathInt(r, "/sriracha/log/p")
	data.Template = "manage_log"
	data.Manage.Logs = db.LogsByPage(page)
	data.Page = page
	data.Pages = pageCount(db.LogCount(), logPageSize)
}
