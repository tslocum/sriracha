package sriracha

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addBoard(b *Board) {
	var reports int
	if b.Reports {
		reports = 1
	}
	_, err := db.conn.Exec(context.Background(), "INSERT INTO board VALUES (DEFAULT, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)",
		b.Dir,
		b.Name,
		b.Description,
		b.Type,
		b.Lock,
		b.Approval,
		reports,
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
		b.DefaultName,
		b.WordBreak,
		b.Truncate,
		b.MaxSize,
		b.ThumbWidth,
		b.ThumbHeight,
	)
	if err != nil {
		log.Fatalf("failed to insert board: %s", err)
	}
	err = db.conn.QueryRow(context.Background(), "SELECT id FROM board WHERE dir = $1", b.Dir).Scan(&b.ID)
	if err != nil || b.ID == 0 {
		log.Fatalf("failed to select id of inserted board: %s", err)
	}
	for _, upload := range b.Uploads {
		_, err := db.conn.Exec(context.Background(), "INSERT INTO board_upload VALUES ($1, $2)", b.ID, upload)
		if err != nil {
			log.Fatalf("failed to insert board uploads: %s", err)
		}
	}
	for _, embed := range b.Embeds {
		_, err := db.conn.Exec(context.Background(), "INSERT INTO board_embed VALUES ($1, $2)", b.ID, embed)
		if err != nil {
			log.Fatalf("failed to insert board embeds: %s", err)
		}
	}
}

func (db *Database) setBoardAttributes(b *Board) {
	rows, err := db.conn.Query(context.Background(), "SELECT upload FROM board_upload WHERE board = $1", b.ID)
	if err != nil {
		log.Fatalf("failed to select board uploads: %s", err)
	}
	b.Uploads = nil
	for rows.Next() {
		var mimeType string
		err := rows.Scan(&mimeType)
		if err != nil {
			log.Fatalf("failed to select board uploads: %s", err)
		}
		for _, u := range srirachaServer.config.UploadTypes() {
			if u.MIME == mimeType {
				b.Uploads = append(b.Uploads, u.MIME)
				break
			}
		}
	}

	rows, err = db.conn.Query(context.Background(), "SELECT embed FROM board_embed WHERE board = $1", b.ID)
	if err != nil {
		log.Fatalf("failed to select board embeds: %s", err)
	}
	b.Embeds = nil
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			log.Fatalf("failed to select board embeds: %s", err)
		}
		b.Embeds = append(b.Embeds, name)
	}
}

func (db *Database) boardByID(id int) *Board {
	b := &Board{}
	err := scanBoard(b, db.conn.QueryRow(context.Background(), "SELECT * FROM board WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select board: %s", err)
	}
	db.setBoardAttributes(b)
	return b
}

func (db *Database) boardByDir(dir string) *Board {
	b := &Board{}
	err := scanBoard(b, db.conn.QueryRow(context.Background(), "SELECT * FROM board WHERE dir = $1", dir))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select board: %s", err)
	}
	db.setBoardAttributes(b)
	return b
}

func (db *Database) allBoards() []*Board {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM board ORDER BY dir ASC")
	if err != nil {
		log.Fatalf("failed to select all boards: %s", err)
	}
	var boards []*Board
	for rows.Next() {
		b := &Board{}
		err := scanBoard(b, rows)
		if err != nil {
			log.Fatalf("failed to select all boards: %s", err)
		}
		boards = append(boards, b)
	}
	for _, b := range boards {
		db.setBoardAttributes(b)
	}
	return boards
}

func (db *Database) updateBoard(b *Board) {
	if b.ID <= 0 {
		log.Fatalf("invalid board ID %d", b.ID)
	}
	var reports int
	if b.Reports {
		reports = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE board SET dir = $1, name = $2, description = $3, type = $4, lock = $5, approval = $6, reports = $7, locale = $8, delay = $9, threads = $10, replies = $11, maxname = $12, maxemail = $13, maxsubject = $14, maxmessage = $15, maxthreads = $16, maxreplies = $17, defaultname = $18, wordbreak = $19, truncate = $20, maxsize = $21, thumbwidth = $22, thumbheight = $23 WHERE id = $24",
		b.Dir,
		b.Name,
		b.Description,
		b.Type,
		b.Lock,
		b.Approval,
		reports,
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
		b.DefaultName,
		b.WordBreak,
		b.Truncate,
		b.MaxSize,
		b.ThumbWidth,
		b.ThumbHeight,
		b.ID,
	)
	if err != nil {
		log.Fatalf("failed to update board: %s", err)
	}

	_, err = db.conn.Exec(context.Background(), "DELETE FROM board_upload WHERE board = $1", b.ID)
	if err != nil {
		log.Fatalf("failed to delete board uploads: %s", err)
	}
	for _, upload := range b.Uploads {
		_, err := db.conn.Exec(context.Background(), "INSERT INTO board_upload VALUES ($1, $2)", b.ID, upload)
		if err != nil {
			log.Fatalf("failed to insert board uploads: %s", err)
		}
	}

	_, err = db.conn.Exec(context.Background(), "DELETE FROM board_embed WHERE board = $1", b.ID)
	if err != nil {
		log.Fatalf("failed to delete board embeds: %s", err)
	}
	for _, embed := range b.Embeds {
		_, err := db.conn.Exec(context.Background(), "INSERT INTO board_embed VALUES ($1, $2)", b.ID, embed)
		if err != nil {
			log.Fatalf("failed to insert board embeds: %s", err)
		}
	}
}

func scanBoard(b *Board, row pgx.Row) error {
	var reports int
	err := row.Scan(
		&b.ID,
		&b.Dir,
		&b.Name,
		&b.Description,
		&b.Type,
		&b.Lock,
		&b.Approval,
		&reports,
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
		&b.DefaultName,
		&b.WordBreak,
		&b.Truncate,
		&b.MaxSize,
		&b.ThumbWidth,
		&b.ThumbHeight,
	)
	if err != nil {
		return err
	}
	b.Reports = reports == 1
	return nil
}
