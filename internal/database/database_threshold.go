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
	var anywhere int
	if t.Anywhere {
		anywhere = 1
	}
	err := db.conn.QueryRow(context.Background(), "INSERT INTO threshold VALUES (DEFAULT, $1, $2, $3, $4, $5, $6) RETURNING id",
		everyone,
		t.Amount,
		t.Event,
		anywhere,
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
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM threshold ORDER BY everyone ASC, amount ASC, event ASC, anywhere ASC, duration ASC, action ASC")
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
	var everyone, anywhere int
	if t.Everyone {
		everyone = 1
	}
	if t.Anywhere {
		anywhere = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE threshold SET everyone = $1, amount = $2, event = $3, anywhere = $4, duration = $5, action = $6 WHERE id = $7",
		everyone,
		t.Amount,
		t.Event,
		anywhere,
		t.Duration,
		t.Action,
		t.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update threshold: %w", err))
	}
}

// ThresholdTimeout returns the number of seconds remaining until the provided threshold no longer applies, or zero.
func (db *DB) ThresholdTimeout(t *Threshold, ipHash string, now int64) int {
	query := "SELECT"
	if !t.Anywhere {
		query += " board,"
	}
	query += " COUNT(*) AS num, MIN(timestamp) AS earliest FROM"
	if t.Event == EventPost || t.Event == EventThread {
		query += " post"
	} else {
		query += " report"
	}
	query += " WHERE"
	var extra []string
	var args []any
	if !t.Everyone {
		extra = append(extra, fmt.Sprintf(" ip = $%d", len(args)+1))
		args = append(args, ipHash)
	}
	extra = append(extra, fmt.Sprintf(" timestamp >= $%d", len(args)+1))
	args = append(args, now-int64(t.Duration))
	if t.Event == EventThread {
		extra = append(extra, " parent IS NULL")
	}
	for i, e := range extra {
		if i != 0 {
			query += " AND"
		}
		query += e
	}
	if !t.Anywhere {
		query += " GROUP BY BOARD"
	}
	rows, err := db.conn.Query(context.Background(), query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0
		}
		dbErr(fmt.Errorf("failed to select threshold count: %w", err))
	}
	var results []thresholdResult
	for rows.Next() {
		var r thresholdResult
		var earliest *int64
		if t.Anywhere {
			err = rows.Scan(&r.amount, &earliest)
		} else {
			err = rows.Scan(&r.board, &r.amount, &earliest)
		}
		if err != nil {
			dbErr(fmt.Errorf("failed to scan threshold count: %w", err))
		}
		if earliest != nil {
			r.earliest = *earliest
		}
		results = append(results, r)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select threshold count: %w", rows.Err()))
	}
	for _, r := range results {
		if r.amount >= t.Amount {
			return max(t.Duration-int(now-r.earliest), 1)
		}
	}
	return 0
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
	var everyone, anywhere int
	err := row.Scan(
		&t.ID,
		&everyone,
		&t.Amount,
		&t.Event,
		&anywhere,
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
	t.Anywhere = anywhere == 1
	return nil
}

type thresholdResult struct {
	board    int
	amount   int
	earliest int64
}
