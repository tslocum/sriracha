package sriracha

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addBan(b *Ban) {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO ban VALUES (DEFAULT, $1, $2, $3, $4)",
		b.IP,
		time.Now().Unix(),
		b.Expire,
		b.Reason,
	)
	if err != nil {
		log.Fatalf("failed to insert ban: %s", err)
	}
	err = db.conn.QueryRow(context.Background(), "SELECT id FROM ban WHERE ip = $1", b.IP).Scan(&b.ID)
	if err != nil || b.ID == 0 {
		log.Fatalf("failed to select id of inserted ban: %s", err)
	}
}

func (db *Database) banByID(id int) *Ban {
	b := &Ban{}
	err := scanBan(b, db.conn.QueryRow(context.Background(), "SELECT * FROM ban WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select ban: %s", err)
	}
	return b
}

func (db *Database) banByIP(ip string) *Ban {
	b := &Ban{}
	err := scanBan(b, db.conn.QueryRow(context.Background(), "SELECT * FROM ban WHERE ip = $1", ip))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select ban: %s", err)
	}
	return b
}

func (db *Database) allBans() []*Ban {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM ban ORDER BY timestamp DESC")
	if err != nil {
		log.Fatalf("failed to select all bans: %s", err)
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
	return bans
}

func (db *Database) updateBan(b *Ban) {
	if b.ID <= 0 {
		log.Fatalf("invalid ban ID %d", b.ID)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE ban SET expire = $1, reason = $2 WHERE id = $3",
		b.Expire,
		b.Reason,
		b.ID,
	)
	if err != nil {
		log.Fatalf("failed to update ban: %s", err)
	}
}

func (db *Database) deleteExpiredBans() {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM ban WHERE expire != 0 AND expire <= $1", time.Now().Unix())
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Database) deleteBan(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM ban WHERE id = $1", id)
	if err != nil {
		log.Fatalf("failed to delete ban: %s", err)
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
