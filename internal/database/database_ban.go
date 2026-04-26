package database

import (
	"context"
	"fmt"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddBan(b *Ban) {
	err := db.conn.QueryRow(context.Background(), "INSERT INTO ban VALUES (DEFAULT, $1, $2, $3, $4) RETURNING id",
		b.IP,
		time.Now().Unix(),
		b.Expire,
		b.Reason,
	).Scan(&b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert ban: %w", err))
	}
}

func (db *DB) BanByID(id int) *Ban {
	b := &Ban{}
	err := scanBan(b, db.conn.QueryRow(context.Background(), "SELECT * FROM ban WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select ban: %w", err))
	}
	return b
}

func (db *DB) BanByIP(ip string) *Ban {
	b := &Ban{}
	err := scanBan(b, db.conn.QueryRow(context.Background(), "SELECT * FROM ban WHERE ip = $1", ip))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select ban: %w", err))
	}
	return b
}

func (db *DB) AllBans(rangeOnly bool) []*Ban {
	if db.conn == nil {
		return nil
	}
	var extra string
	if rangeOnly {
		extra = " WHERE ip LIKE 'r %'"
	}
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM ban"+extra+" ORDER BY timestamp DESC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select all bans: %w", err))
	}
	var bans []*Ban
	for rows.Next() {
		b := &Ban{}
		err := scanBan(b, rows)
		if err != nil {
			return nil
		}
		bans = append(bans, b)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all bans: %w", rows.Err()))
	}
	return bans
}

func (db *DB) UpdateBan(b *Ban) {
	if b.ID <= 0 {
		dbErr(fmt.Errorf("invalid ban ID %d", b.ID))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE ban SET expire = $1, reason = $2 WHERE id = $3",
		b.Expire,
		b.Reason,
		b.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update ban: %w", err))
	}
}

func (db *DB) DeleteExpiredBans() int {
	var deleted int
	err := db.conn.QueryRow(context.Background(), "WITH deleted AS (DELETE FROM ban WHERE expire != 0 AND expire <= $1 RETURNING *) SELECT COUNT(*) FROM deleted", time.Now().Unix()).Scan(&deleted)
	if err != nil {
		dbErr(err)
	}
	return deleted
}

func (db *DB) DeleteBan(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM ban WHERE id = $1", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete ban: %w", err))
	}
}

func scanBan(b *Ban, row pgx.Row) error {
	return row.Scan(
		&b.ID,
		&b.IP,
		&b.Timestamp,
		&b.Expire,
		&b.Reason,
	)
}

func (db *DB) AddFileBan(fileHash string) {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO banfile VALUES ($1) ON CONFLICT DO NOTHING", fileHash)
	if err != nil {
		dbErr(fmt.Errorf("failed to ban file: %w", err))
	}
}

func (db *DB) FileBanned(fileHash string) bool {
	var banned bool
	err := db.conn.QueryRow(context.Background(), "SELECT true FROM banfile WHERE hash = $1", fileHash).Scan(&banned)
	if err == pgx.ErrNoRows {
		return false
	} else if err != nil {
		dbErr(fmt.Errorf("failed to check if file is banned: %w", err))
	}
	return banned
}

func (db *DB) LiftFileBan(fileHash string) {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM banfile WHERE hash = $1", fileHash)
	if err != nil {
		dbErr(fmt.Errorf("failed to lift file ban: %w", err))
	}
}
