package sriracha

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addLog(l *Log) {
	if l.Message == "" {
		return
	}

	var accountID *int
	if l.Account != nil {
		accountID = &l.Account.ID
	}
	var boardID *int
	if l.Board != nil {
		boardID = &l.Board.ID
	}
	_, err := db.conn.Exec(context.Background(), "INSERT INTO log VALUES (DEFAULT, $1, $2, $3, $4, $5)",
		accountID,
		boardID,
		time.Now().Unix(),
		l.Message,
		l.Changes,
	)
	if err != nil {
		log.Fatalf("failed to insert log: %s", err)
	}
}

func (db *Database) log(account *Account, board *Board, message string, changes string) {
	db.addLog(&Log{
		Account: account,
		Board:   board,
		Message: message,
		Changes: changes,
	})
}

func (db *Database) logCount() int {
	var count int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM log").Scan(&count)
	if err == pgx.ErrNoRows {
		return 0
	} else if err != nil {
		log.Fatalf("failed to select log count: %s", err)
	}
	return count
}

func (db *Database) logsByPage(page int) []*Log {
	offset := page * logPageSize
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM log ORDER BY id DESC LIMIT $1 OFFSET $2", logPageSize, offset)
	if err != nil {
		log.Fatalf("failed to select all logs: %s", err)
	}
	var logs []*Log
	var accountIDs []int
	var boardIDs []int
	for rows.Next() {
		l := &Log{}
		var boardID *int
		var accountID *int
		err := rows.Scan(&l.ID, &accountID, &boardID, &l.Timestamp, &l.Message, &l.Changes)
		if err != nil {
			log.Fatalf("failed to select all logs: %s", err)
		}
		logs = append(logs, l)
		if accountID == nil {
			accountIDs = append(accountIDs, 0)
		} else {
			accountIDs = append(accountIDs, *accountID)
		}
		if boardID == nil {
			boardIDs = append(boardIDs, 0)
		} else {
			boardIDs = append(boardIDs, *boardID)
		}
	}
	for i, l := range logs {
		accountID := accountIDs[i]
		boardID := boardIDs[i]
		if accountID > 0 {
			l.Account = db.accountByID(accountID)
		}
		if boardID > 0 {
			l.Board = db.BoardByID(boardID)
		}
	}
	return logs
}
