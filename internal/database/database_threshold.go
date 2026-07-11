package database

import (
	"context"
	"fmt"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddThreshold(t *Threshold) {
	var everyone int
	if t.Everyone {
		everyone = 1
	}
	var everywhere int
	if t.Everywhere {
		everywhere = 1
	}
	err := db.conn.QueryRow(context.Background(), "INSERT INTO threshold VALUES (DEFAULT, $1, $2, $3, $4, $5, $6) RETURNING id",
		everyone,
		t.Amount,
		t.Event,
		everywhere,
		t.Duration,
		t.Action).Scan(&t.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert threshold: %w", err))
	}
}

func (db *DB) ThresholdByID(id int) *Threshold {
	s := &Threshold{}
	err := scanThreshold(s, db.conn.QueryRow(context.Background(), "SELECT * FROM threshold WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select threshold: %w", err))
	}
	return s
}

func (db *DB) AllThresholds() []*Threshold {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM threshold ORDER BY everyone ASC, amount ASC, event ASC, everywhere ASC, duration ASC, action ASC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select all thresholds: %w", err))
	}
	var thresholds []*Threshold
	for rows.Next() {
		t := &Threshold{}
		err := scanThreshold(t, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select all thresholds: %w", err))
		}
		thresholds = append(thresholds, t)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all thresholds: %w", rows.Err()))
	}
	return thresholds
}

func (db *DB) UpdateThreshold(t *Threshold) {
	if t.ID <= 0 {
		dbErr(fmt.Errorf("invalid threshold ID %d", t.ID))
	}
	var everyone, everywhere int
	if t.Everyone {
		everyone = 1
	}
	if t.Everywhere {
		everywhere = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE threshold SET everyone = $1, amount = $2, event = $3, everywhere = $4, duration = $5, action = $6 WHERE id = $7",
		everyone,
		t.Amount,
		t.Event,
		everywhere,
		t.Duration,
		t.Action,
		t.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update threshold: %w", err))
	}
}

func (db *DB) DeleteThreshold(id int) {
	if id <= 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM threshold WHERE id = $1", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete threshold: %w", err))
	}
}

func scanThreshold(t *Threshold, row pgx.Row) error {
	var everyone, everywhere int
	err := row.Scan(
		&t.ID,
		&everyone,
		&t.Amount,
		&t.Event,
		&everywhere,
		&t.Duration,
		&t.Action,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return err
		}
		return fmt.Errorf("failed to scan threshold: %w", err)
	}
	t.Everyone = everyone == 1
	t.Everywhere = everywhere == 1
	return nil
}
