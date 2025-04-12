package sriracha

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addKeyword(k *Keyword) error {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO keyword VALUES (DEFAULT, $1, $2)", k.Text, k.Action)
	if err != nil {
		return fmt.Errorf("failed to insert keyword: %s", err)
	}
	err = db.conn.QueryRow(context.Background(), "SELECT id FROM keyword WHERE text = $1", k.Text).Scan(&k.ID)
	if err != nil {
		return fmt.Errorf("failed to select number of super-administrator accounts: %s", err)
	} else if k.ID == 0 {
		log.Fatal("failed to select id of added keyword")
	}
	return db.updateKeywordBoards(k)
}

func (db *Database) fetchKeywordBoards(k *Keyword) error {
	k.Boards = nil

	rows, err := db.conn.Query(context.Background(), "SELECT board FROM keyword_board WHERE keyword = $1", k.ID)
	if err != nil {
		return fmt.Errorf("failed to select keyword boards: %s", err)
	}
	var ids []int
	for rows.Next() {
		var id int
		err := rows.Scan(&id)
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		var err error
		k.Boards, err = db.allBoards()
		return err
	}
	for _, id := range ids {
		b, err := db.boardByID(id)
		if err != nil {
			return err
		}
		k.Boards = append(k.Boards, b)
	}
	return nil
}

func (db *Database) updateKeywordBoards(k *Keyword) error {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM keyword_board WHERE keyword = $1", k.ID)
	if err != nil {
		return fmt.Errorf("failed to update keyword boards: %s", err)
	}
	for _, b := range k.Boards {
		_, err = db.conn.Exec(context.Background(), "INSERT INTO keyword_board VALUES ($1, $2)", k.ID, b.ID)
		if err != nil {
			return fmt.Errorf("failed to update keyword boards: %s", err)
		}
	}
	return nil
}

func (db *Database) keywordByID(id int) (*Keyword, error) {
	k := &Keyword{}
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM keyword WHERE id = $1", id).Scan(&k.ID, &k.Text, &k.Action)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select keyword: %s", err)
	}
	err = db.fetchKeywordBoards(k)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (db *Database) keywordByText(text string) (*Keyword, error) {
	k := &Keyword{}
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM keyword WHERE text = $1", text).Scan(&k.ID, &k.Text, &k.Action)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select keyword: %s", err)
	}
	err = db.fetchKeywordBoards(k)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (db *Database) allKeywords() ([]*Keyword, error) {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM keyword ORDER BY text ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to select all keywords: %s", err)
	}
	var keywords []*Keyword
	for rows.Next() {
		k := &Keyword{}
		err := rows.Scan(&k.ID, &k.Text, &k.Action)
		if err != nil {
			return nil, err
		}
		keywords = append(keywords, k)
	}
	for _, k := range keywords {
		err = db.fetchKeywordBoards(k)
		if err != nil {
			return nil, err
		}
	}
	return keywords, nil
}

func (db *Database) updateKeyword(k *Keyword) error {
	if k.ID <= 0 {
		return fmt.Errorf("invalid keyword ID %d", k.ID)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE keyword SET text = $1, action = $2 WHERE id = $3", k.Text, k.Action, k.ID)
	if err != nil {
		return fmt.Errorf("failed to update keyword: %s", err)
	}
	return db.updateKeywordBoards(k)
}
