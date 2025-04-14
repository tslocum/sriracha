package sriracha

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addBoard(b *Board) error {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO board VALUES (DEFAULT, $1, $2, $3, $4, $5, $6, $7, $8)", b.Dir, b.Name, b.Description, b.Type, b.Approval, b.MaxSize, b.ThumbWidth, b.ThumbHeight)
	if err != nil {
		return fmt.Errorf("failed to insert board: %s", err)
	}
	err = db.conn.QueryRow(context.Background(), "SELECT id FROM board WHERE dir = $1", b.Dir).Scan(&b.ID)
	if err != nil || b.ID == 0 {
		return fmt.Errorf("failed to select id of inserted board: %s", err)
	}
	return nil
}

func (db *Database) boardByID(id int) (*Board, error) {
	b := &Board{}
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM board WHERE id = $1", id).Scan(&b.ID, &b.Dir, &b.Name, &b.Description, &b.Type, &b.Approval, &b.MaxSize, &b.ThumbWidth, &b.ThumbHeight)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select board: %s", err)
	}
	return b, nil
}

func (db *Database) allBoards() ([]*Board, error) {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM board ORDER BY dir ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to select all boards: %s", err)
	}
	var boards []*Board
	for rows.Next() {
		b := &Board{}
		err := rows.Scan(&b.ID, &b.Dir, &b.Name, &b.Description, &b.Type, &b.Approval, &b.MaxSize, &b.ThumbWidth, &b.ThumbHeight)
		if err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, nil
}

func (db *Database) updateBoard(b *Board) error {
	if b.ID <= 0 {
		return fmt.Errorf("invalid board ID %d", b.ID)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE board SET dir = $1, name = $2, description = $3, type = $4, approval = $5, maxsize = $6, thumbwidth = $7, thumbheight = $8 WHERE id = $9", b.Dir, b.Name, b.Description, b.Type, b.Approval, b.MaxSize, b.ThumbWidth, b.ThumbHeight, b.ID)
	if err != nil {
		return fmt.Errorf("failed to update board: %s", err)
	}
	return nil
}
