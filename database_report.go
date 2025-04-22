package sriracha

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addReport(r *Report) {
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
		log.Fatalf("failed to insert report: %s", err)
	}
}

func (db *Database) allReports() []*Report {
	rows, err := db.conn.Query(context.Background(), "SELECT DISTINCT(board, post) FROM report")
	if err != nil {
		log.Fatalf("failed to select all reports: %s", err)
	}
	var distinctIDs [][2]int
	for rows.Next() {
		colValues, err := rows.Values()
		if err != nil {
			log.Fatal(err)
		}
		var ids [2]int
		for _, colValue := range colValues {
			for i, v := range colValue.([]interface{}) {
				ids[i] = parseInt(fmt.Sprintf("%d", v)) // Type may be int16 or int32.
			}
		}
		distinctIDs = append(distinctIDs, ids)
	}

	reports := make([]*Report, len(distinctIDs))
	for i, ids := range distinctIDs {
		boardID := ids[0]
		postID := ids[1]

		r := &Report{}
		r.Board = db.boardByID(boardID)
		r.Post = db.postByID(postID)
		r.count = db.numReports(r.Post)
		reports[i] = r
	}
	return reports
}

func (db *Database) numReports(p *Post) int {
	var count int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM report WHERE board = $1 AND post = $2", p.Board.ID, p.ID).Scan(&count)
	if err == pgx.ErrNoRows {
		return 0
	} else if err != nil {
		log.Fatalf("failed to select report count: %s", err)
	}
	return count
}

func (db *Database) deleteReports(p *Post) {
	if p.ID == 0 || p.Board == nil {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM report WHERE board = $1 AND post = $2", p.Board.ID, p.ID)
	if err != nil {
		log.Fatalf("failed to delete reports: %s", err)
	}
}
