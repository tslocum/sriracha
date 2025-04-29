package sriracha

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addKeyword(k *Keyword) {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO keyword VALUES (DEFAULT, $1, $2)",
		k.Text,
		k.Action,
	)
	if err != nil {
		log.Fatalf("failed to insert keyword: %s", err)
	}
	err = db.conn.QueryRow(context.Background(), "SELECT id FROM keyword WHERE text = $1", k.Text).Scan(&k.ID)
	if err != nil {
		log.Fatalf("failed to select number of super-administrator accounts: %s", err)
	} else if k.ID == 0 {
		log.Fatal("failed to select id of added keyword")
	}
	db.updateKeywordBoards(k)
}

func (db *Database) fetchKeywordBoards(k *Keyword) {
	k.Boards = nil

	rows, err := db.conn.Query(context.Background(), "SELECT board FROM keyword_board WHERE keyword = $1", k.ID)
	if err != nil {
		log.Fatalf("failed to select keyword boards: %s", err)
	}
	var ids []int
	for rows.Next() {
		var id int
		err := rows.Scan(&id)
		if err != nil {
			log.Fatalf("failed to select keyword boards: %s", err)
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		k.Boards = db.allBoards()
		return
	}
	for _, id := range ids {
		b := db.boardByID(id)
		k.Boards = append(k.Boards, b)
	}
}

func (db *Database) updateKeywordBoards(k *Keyword) {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM keyword_board WHERE keyword = $1", k.ID)
	if err != nil {
		log.Fatalf("failed to update keyword boards: %s", err)
	}
	for _, b := range k.Boards {
		_, err = db.conn.Exec(context.Background(), "INSERT INTO keyword_board VALUES ($1, $2)", k.ID, b.ID)
		if err != nil {
			log.Fatalf("failed to update keyword boards: %s", err)
		}
	}
}

func (db *Database) keywordByID(id int) *Keyword {
	k := &Keyword{}
	err := scanKeyword(k, db.conn.QueryRow(context.Background(), "SELECT * FROM keyword WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select keyword: %s", err)
	}
	db.fetchKeywordBoards(k)
	return k
}

func (db *Database) keywordByText(text string) *Keyword {
	k := &Keyword{}
	err := scanKeyword(k, db.conn.QueryRow(context.Background(), "SELECT * FROM keyword WHERE text = $1", text))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select keyword: %s", err)
	}
	db.fetchKeywordBoards(k)
	return k
}

func (db *Database) allKeywords() []*Keyword {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM keyword ORDER BY text ASC")
	if err != nil {
		log.Fatalf("failed to select all keywords: %s", err)
	}
	var keywords []*Keyword
	for rows.Next() {
		k := &Keyword{}
		err := scanKeyword(k, rows)
		if err != nil {
			log.Fatalf("failed to select all keywords: %s", err)
		}
		keywords = append(keywords, k)
	}
	for _, k := range keywords {
		db.fetchKeywordBoards(k)
	}
	return keywords
}

func (db *Database) updateKeyword(k *Keyword) {
	if k.ID <= 0 {
		log.Fatalf("invalid keyword ID %d", k.ID)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE keyword SET text = $1, action = $2 WHERE id = $3",
		k.Text,
		k.Action,
		k.ID,
	)
	if err != nil {
		log.Fatalf("failed to update keyword: %s", err)
	}
	db.updateKeywordBoards(k)
}

func (db *Database) deleteKeyword(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM keyword WHERE id = $1", id)
	if err != nil {
		log.Fatalf("failed to delete keyword: %s", err)
	}
}

func scanKeyword(k *Keyword, row pgx.Row) error {
	return row.Scan(
		&k.ID,
		&k.Text,
		&k.Action,
	)
}
