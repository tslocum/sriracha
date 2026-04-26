package database

import (
	"context"
	"fmt"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddKeyword(k *Keyword) {
	err := db.conn.QueryRow(context.Background(), "INSERT INTO keyword VALUES (DEFAULT, $1, $2) RETURNING id",
		k.Text,
		k.Action,
	).Scan(&k.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert keyword: %w", err))
	}
	db.updateKeywordBoards(k)
}

func (db *DB) fetchKeywordBoards(k *Keyword) {
	k.Boards = nil

	rows, err := db.conn.Query(context.Background(), "SELECT board FROM keyword_board WHERE keyword = $1", k.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to select keyword boards: %w", err))
	}
	var ids []int
	for rows.Next() {
		var id int
		err := rows.Scan(&id)
		if err != nil {
			dbErr(fmt.Errorf("failed to select keyword boards: %w", err))
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select keyword boards: %w", rows.Err()))
	}

	for _, id := range ids {
		b := db.BoardByID(id)
		k.Boards = append(k.Boards, b)
	}
}

func (db *DB) updateKeywordBoards(k *Keyword) {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM keyword_board WHERE keyword = $1", k.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update keyword boards: %w", err))
	}
	for _, b := range k.Boards {
		_, err = db.conn.Exec(context.Background(), "INSERT INTO keyword_board VALUES ($1, $2)", k.ID, b.ID)
		if err != nil {
			dbErr(fmt.Errorf("failed to update keyword boards: %w", err))
		}
	}
}

func (db *DB) KeywordByID(id int) *Keyword {
	k := &Keyword{}
	err := scanKeyword(k, db.conn.QueryRow(context.Background(), "SELECT * FROM keyword WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select keyword: %w", err))
	}
	db.fetchKeywordBoards(k)
	return k
}

func (db *DB) KeywordByText(text string) *Keyword {
	k := &Keyword{}
	err := scanKeyword(k, db.conn.QueryRow(context.Background(), "SELECT * FROM keyword WHERE text = $1", text))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select keyword: %w", err))
	}
	db.fetchKeywordBoards(k)
	return k
}

func (db *DB) AllKeywords() []*Keyword {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM keyword ORDER BY text ASC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select all keywords: %w", err))
	}
	var keywords []*Keyword
	for rows.Next() {
		k := &Keyword{}
		err := scanKeyword(k, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select all keywords: %w", err))
		}
		keywords = append(keywords, k)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all keywords: %w", rows.Err()))
	}
	for _, k := range keywords {
		db.fetchKeywordBoards(k)
	}
	return keywords
}

func (db *DB) UpdateKeyword(k *Keyword) {
	if k.ID <= 0 {
		dbErr(fmt.Errorf("invalid keyword ID %d", k.ID))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE keyword SET text = $1, action = $2 WHERE id = $3",
		k.Text,
		k.Action,
		k.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update keyword: %w", err))
	}
	db.updateKeywordBoards(k)
}

func (db *DB) DeleteKeyword(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM keyword WHERE id = $1", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete keyword: %w", err))
	}
}

func scanKeyword(k *Keyword, row pgx.Row) error {
	return row.Scan(
		&k.ID,
		&k.Text,
		&k.Action,
	)
}
