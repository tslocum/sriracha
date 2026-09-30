package database

import (
	"context"
	"fmt"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddBan(b *Ban) {
	err := db.conn.QueryRow(context.Background(), "INSERT INTO ban VALUES (DEFAULT, $1, $2, $3, $4, $5, $6) RETURNING id",
		b.IP,
		time.Now().Unix(),
		b.Expire,
		b.Reason,
		b.LiftedTimestamp,
		b.LiftedReason,
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
	err := scanBan(b, db.conn.QueryRow(context.Background(), "SELECT * FROM ban WHERE ip = $1 AND liftedtimestamp = 0", ip))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select ban: %w", err))
	}
	return b
}

func (db *DB) AllActiveBans(rangeOnly bool) []*Ban {
	if db.conn == nil {
		return nil
	}
	var extra string
	if rangeOnly {
		extra = " AND ip LIKE 'r %'"
	}
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM ban WHERE liftedtimestamp = 0"+extra+" ORDER BY timestamp DESC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select active bans: %w", err))
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
		dbErr(fmt.Errorf("failed to select active bans: %w", rows.Err()))
	}
	return bans
}

func (db *DB) LiftedBansByIP(ipHash string) []*Ban {
	if db.conn == nil {
		return nil
	}
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM ban WHERE ip = $1 AND liftedtimestamp != 0 ORDER BY timestamp DESC", ipHash)
	if err != nil {
		dbErr(fmt.Errorf("failed to select lifted bans: %w", err))
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
		dbErr(fmt.Errorf("failed to select lifted bans: %w", rows.Err()))
	}
	return bans
}

func (db *DB) UpdateBan(b *Ban) {
	if b.ID <= 0 {
		dbErr(fmt.Errorf("invalid ban ID %d", b.ID))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE ban SET expire = $1, reason = $2, liftedtimestamp = $3, liftedreason = $4 WHERE id = $5",
		b.Expire,
		b.Reason,
		b.LiftedTimestamp,
		b.LiftedReason,
		b.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update ban: %w", err))
	}
}

func (db *DB) LiftBan(id int, reason string) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE ban SET liftedtimestamp = $1, liftedreason = $2 WHERE id = $3 AND liftedtimestamp = 0", time.Now().Unix(), reason, id)
	if err != nil {
		dbErr(fmt.Errorf("failed to lift ban: %w", err))
	}
}

func (db *DB) LiftExpiredBans() []int {
	rows, err := db.conn.Query(context.Background(), "WITH processed AS (UPDATE ban SET liftedtimestamp = $1, liftedreason = $2 WHERE liftedtimestamp = 0 AND expire != 0 AND expire <= $1 RETURNING *) SELECT id FROM processed", time.Now().Unix(), Get(nil, nil, "Expired")+".")
	if err != nil {
		dbErr(fmt.Errorf("failed to select expired bans: %w", err))
	}
	var ids []int
	for rows.Next() {
		var id int
		err := rows.Scan(&id)
		if err != nil {
			dbErr(fmt.Errorf("failed to scan expired ban ID: %w", rows.Err()))
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select expired bans: %w", rows.Err()))
	}
	return ids
}

func scanBan(b *Ban, row pgx.Row) error {
	return row.Scan(
		&b.ID,
		&b.IP,
		&b.Timestamp,
		&b.Expire,
		&b.Reason,
		&b.LiftedTimestamp,
		&b.LiftedReason,
	)
}

func (db *DB) AddBanAppeal(a *BanAppeal) {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO banappeal VALUES (DEFAULT, $1, $2, $3, $4, $5, $6)",
		a.Ban.ID,
		a.Timestamp,
		a.Reason,
		a.Outcome,
		a.OutcomeTimestamp,
		a.OutcomeReason,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to add ban appeal: %w", err))
	}
}

func (db *DB) BanAppealByID(id int) *BanAppeal {
	a := &BanAppeal{}
	banID, err := scanBanAppeal(a, db.conn.QueryRow(context.Background(), "SELECT * FROM banappeal WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select ban: %w", err))
	}
	a.Ban = db.BanByID(banID)
	return a
}

func (db *DB) PendingBanAppeals() []*BanAppeal {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM banappeal WHERE outcome = 0 ORDER BY id ASC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select pending ban appeals: %w", err))
	}
	var appeals []*BanAppeal
	var banIDs []int
	for rows.Next() {
		a := &BanAppeal{}
		banID, err := scanBanAppeal(a, rows)
		if err == pgx.ErrNoRows {
			return nil
		} else if err != nil {
			dbErr(fmt.Errorf("failed to select ban appeal: %w", err))
		}
		appeals = append(appeals, a)
		banIDs = append(banIDs, banID)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select pending ban appeals: %w", rows.Err()))
	}
	for i := range appeals {
		appeals[i].Ban = db.BanByID(banIDs[i])
	}
	return appeals
}

func (db *DB) BanAppeals(b *Ban) []*BanAppeal {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM banappeal WHERE ban = $1 ORDER BY id ASC", b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to select ban appeals: %w", err))
	}
	var appeals []*BanAppeal
	for rows.Next() {
		a := &BanAppeal{}
		_, err := scanBanAppeal(a, rows)
		if err == pgx.ErrNoRows {
			return nil
		} else if err != nil {
			dbErr(fmt.Errorf("failed to select ban appeal: %w", err))
		}
		appeals = append(appeals, a)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select ban appeals: %w", rows.Err()))
	}
	for i := range appeals {
		appeals[i].Ban = b
	}
	return appeals
}

func (db *DB) UpdateBanAppeal(a *BanAppeal) {
	if a.ID <= 0 {
		dbErr(fmt.Errorf("invalid ban appeal ID %d", a.ID))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE banappeal SET outcome = $1, outcometimestamp = $2, outcomereason = $3 WHERE id = $4",
		a.Outcome,
		a.OutcomeTimestamp,
		a.OutcomeReason,
		a.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update ban appeal: %w", err))
	}
}

func scanBanAppeal(a *BanAppeal, row pgx.Row) (int, error) {
	var banID int
	err := row.Scan(
		&a.ID,
		&banID,
		&a.Timestamp,
		&a.Reason,
		&a.Outcome,
		&a.OutcomeTimestamp,
		&a.OutcomeReason,
	)
	return banID, err
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
