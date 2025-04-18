package sriracha

import (
	"context"
	"fmt"
	"log"
	"time"
)

func (db *Database) addLog(l *Log) {
	if l.Message == "" {
		return
	}

	var boardID *int
	if l.Board != nil {
		boardID = &l.Board.ID
	}
	var accountID *int
	if l.Account != nil {
		accountID = &l.Account.ID
	}
	_, err := db.conn.Exec(context.Background(), "INSERT INTO log VALUES (DEFAULT, $1, $2, $3, $4, $5)",
		boardID,
		time.Now().Unix(),
		accountID,
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

func (db *Database) allLogs() ([]*Log, error) {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM log ORDER BY id DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to select all logs: %s", err)
	}
	var logs []*Log
	var boardIDs []int
	var accountIDs []int
	for rows.Next() {
		l := &Log{}
		var boardID *int
		var accountID *int
		err := rows.Scan(&l.ID, &boardID, &l.Timestamp, &accountID, &l.Message, &l.Changes)
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
		if boardID == nil {
			boardIDs = append(boardIDs, 0)
		} else {
			boardIDs = append(boardIDs, *boardID)
		}
		if accountID == nil {
			accountIDs = append(accountIDs, 0)
		} else {
			accountIDs = append(accountIDs, *accountID)
		}
	}
	for i, l := range logs {
		boardID := boardIDs[i]
		accountID := accountIDs[i]
		if boardID > 0 {
			l.Board, err = db.boardByID(boardID)
			if err != nil {
				return nil, err
			}
		}
		if accountID > 0 {
			l.Account, err = db.accountByID(accountID)
			if err != nil {
				return nil, err
			}
		}
	}
	return logs, nil
}
