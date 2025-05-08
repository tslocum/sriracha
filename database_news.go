package sriracha

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addNews(n *News) {
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
		log.Fatalf("failed to insert news: %s", err)
	}
}

func (db *Database) newsByID(id int) *News {
	n := &News{}
	accountID, err := scanNews(n, db.conn.QueryRow(context.Background(), "SELECT * FROM news WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select news: %s", err)
	} else if accountID != 0 {
		n.Account = db.accountByID(accountID)
	}
	return n
}

func (db *Database) allNews(onlyPublished bool) []*News {
	var rows pgx.Rows
	var err error
	if onlyPublished {
		rows, err = db.conn.Query(context.Background(), "SELECT * FROM news WHERE timestamp != 0 AND timestamp <= $1 ORDER BY timestamp DESC", time.Now().Unix())
	} else {
		rows, err = db.conn.Query(context.Background(), "SELECT * FROM news ORDER BY timestamp = 0, timestamp DESC")
	}
	if err != nil {
		log.Fatalf("failed to select all news: %s", err)
	}
	var news []*News
	var accountIDs []int
	for rows.Next() {
		n := &News{}
		accountID, err := scanNews(n, rows)
		if err != nil {
			log.Fatalf("failed to select all news: %s", err)
		}
		news = append(news, n)
		accountIDs = append(accountIDs, accountID)
	}
	for i, n := range news {
		if accountIDs[i] == 0 {
			continue
		}
		n.Account = db.accountByID(accountIDs[i])
	}
	return news
}

func (db *Database) updateNews(n *News) {
	if n.ID <= 0 {
		log.Fatalf("invalid news ID %d", n.ID)
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
		log.Fatalf("failed to update news: %s", err)
	}
}

func (db *Database) deleteNews(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM news WHERE id = $1", id)
	if err != nil {
		log.Fatalf("failed to delete news: %s", err)
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
