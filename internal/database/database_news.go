package database

import (
	"context"
	"fmt"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddNews(n *News) {
	var accountID *int
	if n.Account != nil {
		accountID = &n.Account.ID
	}
	var share int
	if n.Share {
		share = 1
	}
	err := db.conn.QueryRow(context.Background(), "INSERT INTO news VALUES (DEFAULT, $1, $2, $3, $4, $5, $6, $7) RETURNING id",
		accountID,
		n.Timestamp,
		time.Now().Unix(),
		share,
		n.Name,
		n.Subject,
		n.Message).Scan(&n.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert news: %w", err))
	}
}

func (db *DB) NewsByID(id int) *News {
	n := &News{}
	accountID, err := scanNews(n, db.conn.QueryRow(context.Background(), "SELECT * FROM news WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select news: %w", err))
	} else if accountID != 0 {
		n.Account = db.AccountByID(accountID)
	}
	return n
}

func (db *DB) AllNews(onlyPublished bool) []*News {
	var rows pgx.Rows
	var err error
	if onlyPublished {
		rows, err = db.conn.Query(context.Background(), "SELECT * FROM news WHERE timestamp != 0 AND timestamp <= $1 ORDER BY timestamp DESC", time.Now().Unix())
	} else {
		rows, err = db.conn.Query(context.Background(), "SELECT * FROM news ORDER BY timestamp = 0, timestamp DESC")
	}
	if err != nil {
		dbErr(fmt.Errorf("failed to select all news: %w", err))
	}
	var news []*News
	var accountIDs []int
	for rows.Next() {
		n := &News{}
		accountID, err := scanNews(n, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select all news: %w", err))
		}
		news = append(news, n)
		accountIDs = append(accountIDs, accountID)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all news: %w", rows.Err()))
	}
	for i, n := range news {
		if accountIDs[i] == 0 {
			continue
		}
		n.Account = db.AccountByID(accountIDs[i])
	}
	return news
}

func (db *DB) UpdateNews(n *News) {
	if n.ID <= 0 {
		dbErr(fmt.Errorf("invalid news ID %d", n.ID))
	}
	var share int
	if n.Share {
		share = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE news SET timestamp = $1, modified = $2, share = $3, name = $4, subject = $5, message = $6 WHERE id = $7",
		n.Timestamp,
		time.Now().Unix(),
		share,
		n.Name,
		n.Subject,
		n.Message,
		n.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update news: %w", err))
	}
}

func (db *DB) DeleteNews(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM news WHERE id = $1", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete news: %w", err))
	}
}

func scanNews(n *News, row pgx.Row) (int, error) {
	var accountID int
	var share int
	err := row.Scan(
		&n.ID,
		&accountID,
		&n.Timestamp,
		&n.Modified,
		&share,
		&n.Name,
		&n.Subject,
		&n.Message,
	)
	if share == 1 {
		n.Share = true
	}
	return accountID, err
}
