package database

import (
	"context"
	"fmt"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddReport(r *Report) {
	if r.Board == nil || r.Post == nil {
		return
	}
	_, err := db.conn.Exec(context.Background(), "INSERT INTO report VALUES (DEFAULT, $1, $2, $3, $4) ON CONFLICT DO NOTHING",
		r.Board.ID,
		r.Post.ID,
		r.Timestamp,
		r.IP,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert report: %w", err))
	}
}

func (db *DB) AllReports() []*Report {
	rows, err := db.conn.Query(context.Background(), "SELECT DISTINCT(board, post) FROM report")
	if err != nil {
		dbErr(fmt.Errorf("failed to select all reports: %w", err))
	}
	var distinctIDs [][2]int
	for rows.Next() {
		colValues, err := rows.Values()
		if err != nil {
			dbErr(err)
		}
		var ids [2]int
		for _, colValue := range colValues {
			for i, v := range colValue.([]interface{}) {
				ids[i] = ParseInt(fmt.Sprintf("%d", v)) // Type may be int16 or int32.
			}
		}
		distinctIDs = append(distinctIDs, ids)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all reports: %w", rows.Err()))
	}

	reports := make([]*Report, len(distinctIDs))
	for i, ids := range distinctIDs {
		boardID := ids[0]
		postID := ids[1]

		r := &Report{}
		r.Board = db.BoardByID(boardID)
		r.Post = db.PostByID(postID)
		r.Count = db.NumReports(r.Post)
		reports[i] = r
	}
	return reports
}

func (db *DB) NumReports(p *Post) int {
	var count int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM report WHERE board = $1 AND post = $2", p.Board.ID, p.ID).Scan(&count)
	if err == pgx.ErrNoRows {
		return 0
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select report count: %w", err))
	}
	return count
}

func (db *DB) DeleteReports(p *Post) {
	if p.ID == 0 || p.Board == nil {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM report WHERE board = $1 AND post = $2", p.Board.ID, p.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete reports: %w", err))
	}
}
