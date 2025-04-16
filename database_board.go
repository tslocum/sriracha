package sriracha

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addBoard(b *Board) error {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO board VALUES (DEFAULT, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)",
		b.Dir,
		b.Name,
		b.Description,
		b.Type,
		b.Lock,
		b.Approval,
		b.Locale,
		b.Delay,
		b.Threads,
		b.Replies,
		b.MaxName,
		b.MaxEmail,
		b.MaxSubject,
		b.MaxMessage,
		b.MaxThreads,
		b.MaxReplies,
		b.WordBreak,
		b.Truncate,
		b.MaxSize,
		b.ThumbWidth,
		b.ThumbHeight,
	)
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
	err := scanBoard(b, db.conn.QueryRow(context.Background(), "SELECT * FROM board WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select board: %s", err)
	}
	return b, nil
}

func (db *Database) boardByDir(dir string) (*Board, error) {
	b := &Board{}
	err := scanBoard(b, db.conn.QueryRow(context.Background(), "SELECT * FROM board WHERE dir = $1", dir))
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
		err := scanBoard(b, rows)
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
	_, err := db.conn.Exec(context.Background(), "UPDATE board SET dir = $1, name = $2, description = $3, type = $4, lock = $5, approval = $6, locale = $7, delay = $8, threads = $9, replies = $10, maxname = $11, maxemail = $12, maxsubject = $13, maxmessage = $14, maxthreads = $15, maxreplies = $16, wordbreak = $17, truncate = $18, maxsize = $19, thumbwidth = $20, thumbheight = $21 WHERE id = $22",
		b.Dir,
		b.Name,
		b.Description,
		b.Type,
		b.Lock,
		b.Approval,
		b.Locale,
		b.Delay,
		b.Threads,
		b.Replies,
		b.MaxName,
		b.MaxEmail,
		b.MaxSubject,
		b.MaxMessage,
		b.MaxThreads,
		b.MaxReplies,
		b.WordBreak,
		b.Truncate,
		b.MaxSize,
		b.ThumbWidth,
		b.ThumbHeight,
		b.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update board: %s", err)
	}
	return nil
}

func scanBoard(b *Board, row pgx.Row) error {
	return row.Scan(
		&b.ID,
		&b.Dir,
		&b.Name,
		&b.Description,
		&b.Type,
		&b.Lock,
		&b.Approval,
		&b.Locale,
		&b.Delay,
		&b.Threads,
		&b.Replies,
		&b.MaxName,
		&b.MaxEmail,
		&b.MaxSubject,
		&b.MaxMessage,
		&b.MaxThreads,
		&b.MaxReplies,
		&b.WordBreak,
		&b.Truncate,
		&b.MaxSize,
		&b.ThumbWidth,
		&b.ThumbHeight,
	)
}
