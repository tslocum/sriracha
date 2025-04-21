package sriracha

import (
	"context"
	"log"
)

func (db *Database) addReport(r *Report) {
	if r.Board == nil || r.Post == nil {
		return
	}
	_, err := db.conn.Exec(context.Background(), "INSERT INTO report VALUES (DEFAULT, $1, $2, $3, $4)",
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
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM report ORDER BY id ASC")
	if err != nil {
		log.Fatalf("failed to select all reports: %s", err)
	}
	var reports []*Report
	var boardIDs []int
	var postIDs []int
	for rows.Next() {
		r := &Report{}
		var boardID *int
		var postID *int
		err := rows.Scan(&r.ID, &boardID, &postID, &r.Timestamp, &r.IP)
		if err != nil {
			log.Fatalf("failed to select all reports: %s", err)
		}
		reports = append(reports, r)
		if boardID == nil {
			boardIDs = append(boardIDs, 0)
		} else {
			boardIDs = append(boardIDs, *boardID)
		}
		if postIDs == nil {
			postIDs = append(postIDs, 0)
		} else {
			postIDs = append(postIDs, *postID)
		}
	}
	for i, r := range reports {
		boardID := boardIDs[i]
		postID := postIDs[i]
		if boardID > 0 {
			r.Board = db.boardByID(boardID)
		}
		if postID > 0 {
			r.Post = db.postByID(r.Board, postID)
		}
	}
	return reports
}
