package database

import (
	"context"
	"fmt"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddTwoFactor(t *TwoFactor) {
	err := db.conn.QueryRow(context.Background(), "INSERT INTO twofactor VALUES (DEFAULT, $1, $2, $3, $4, $5) RETURNING id",
		t.Account,
		t.Timestamp,
		time.Now().Unix(),
		t.Secret,
		t.Name).Scan(&t.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert two-factor device: %w", err))
	}
}

func (db *DB) TwoFactorByID(id int) *TwoFactor {
	t := &TwoFactor{}
	err := scanTwoFactor(t, db.conn.QueryRow(context.Background(), "SELECT * FROM twofactor WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select two-factor device: %w", err))
	}
	return t
}

func (db *DB) TwoFactorsByAccount(accountID int) []*TwoFactor {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM twofactor WHERE account = $1 ORDER BY lastactive DESC, name ASC, id DESC", accountID)
	if err != nil {
		dbErr(fmt.Errorf("failed to select all two-factor devices: %w", err))
	}
	var devices []*TwoFactor
	for rows.Next() {
		t := &TwoFactor{}
		err := scanTwoFactor(t, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select all two-factor devices: %w", err))
		}
		devices = append(devices, t)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all two-factor devices: %w", rows.Err()))
	}
	return devices
}

func (db *DB) UpdateTwoFactor(t *TwoFactor) {
	if t.ID <= 0 {
		dbErr(fmt.Errorf("invalid two-factor device ID %d", t.ID))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE twofactor SET lastactive = $1, name = $2 WHERE id = $3",
		t.LastActive,
		t.Name,
		t.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update two-factor device: %w", err))
	}
}

func (db *DB) DeleteTwoFactor(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM twofactor WHERE id = $1", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete two-factor device: %w", err))
	}
}

func scanTwoFactor(t *TwoFactor, row pgx.Row) error {
	return row.Scan(
		&t.ID,
		&t.Account,
		&t.Timestamp,
		&t.LastActive,
		&t.Secret,
		&t.Name,
	)
}
