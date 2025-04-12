package sriracha

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addBan(b *Ban) error {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO ban VALUES (DEFAULT, $1, $2, $3, $4)", b.IP, time.Now().Unix(), b.Expire, b.Reason)
	if err != nil {
		return fmt.Errorf("failed to insert ban: %s", err)
	}
	return nil
}

func (db *Database) banByID(id int) (*Ban, error) {
	b := &Ban{}
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM ban WHERE id = $1", id).Scan(&b.ID, &b.IP, &b.Timestamp, &b.Expire, &b.Reason)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select ban: %s", err)
	}
	return b, nil
}

func (db *Database) banByIP(ip string) (*Ban, error) {
	b := &Ban{}
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM ban WHERE ip = $1", ip).Scan(&b.ID, &b.IP, &b.Timestamp, &b.Expire, &b.Reason)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select ban: %s", err)
	}
	return b, nil
}

func (db *Database) allBans() ([]*Ban, error) {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM ban ORDER BY timestamp DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to select all bans: %s", err)
	}
	var bans []*Ban
	for rows.Next() {
		b := &Ban{}
		err := rows.Scan(&b.ID, &b.IP, &b.Timestamp, &b.Expire, &b.Reason)
		if err != nil {
			return nil, err
		}
		bans = append(bans, b)
	}
	return bans, nil
}

func (db *Database) updateBan(b *Ban) error {
	if b.ID <= 0 {
		return fmt.Errorf("invalid ban ID %d", b.ID)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE ban SET expire = $1, reason = $2 WHERE id = $3", b.Expire, b.Reason, b.ID)
	if err != nil {
		return fmt.Errorf("failed to update ban: %s", err)
	}
	return nil
}
