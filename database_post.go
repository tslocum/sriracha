package sriracha

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addPost(p *Post) {
	var parent *int
	if p.Parent != 0 {
		parent = &p.Parent
	}
	err := db.conn.QueryRow(context.Background(), "INSERT INTO post VALUES (DEFAULT, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24) RETURNING id",
		p.Board.ID,
		parent,
		p.Timestamp,
		p.Bumped,
		p.IP,
		p.Name,
		p.Tripcode,
		p.Email,
		p.NameBlock,
		p.Subject,
		p.Message,
		p.Password,
		p.File,
		p.FileHash,
		p.FileOriginal,
		p.FileSize,
		p.FileWidth,
		p.FileHeight,
		p.Thumb,
		p.ThumbWidth,
		p.ThumbHeight,
		p.Moderated,
		p.Stickied,
		p.Locked,
	).Scan(&p.ID)
	if err != nil || p.ID == 0 {
		log.Fatalf("failed to insert post: %s", err)
	}
}

func (db *Database) allThreads(board *Board, moderated bool) []*Post {
	var extra string
	if moderated {
		extra = " AND moderated > 0"
	}
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM post WHERE board = $1 AND parent IS NULL"+extra+" ORDER BY bumped DESC", board.ID)
	if err != nil {
		log.Fatalf("failed to select all posts: %s", err)
	}
	var posts []*Post
	for rows.Next() {
		p := &Post{}
		err := scanPost(p, rows)
		if err != nil {
			log.Fatal(err)
		}
		p.Board = board
		posts = append(posts, p)
	}
	return posts
}

func (db *Database) allPostsInThread(board *Board, postID int, moderated bool) []*Post {
	var extra string
	if moderated {
		extra = " AND moderated > 0"
	}
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM post WHERE board = $1 AND (id = $2 OR parent = $2)"+extra+" ORDER BY id ASC", board.ID, postID)
	if err != nil {
		log.Fatalf("failed to select all posts: %s", err)
	}
	var posts []*Post
	for rows.Next() {
		p := &Post{}
		err := scanPost(p, rows)
		if err != nil {
			log.Fatal(err)
		}
		p.Board = board
		posts = append(posts, p)
	}
	return posts
}

func (db *Database) postByID(board *Board, postID int) *Post {
	p := &Post{}
	err := scanPost(p, db.conn.QueryRow(context.Background(), "SELECT * FROM post WHERE board = $1 AND id = $2", board.ID, postID))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil || p.ID == 0 {
		log.Fatalf("failed to select keyword: %s", err)
	}
	p.Board = board
	return p
}

func (db *Database) bumpThread(threadID int, timestamp int64) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET bumped = $1 WHERE id = $2", timestamp, threadID)
	if err != nil {
		log.Fatalf("failed to update account: %s", err)
	}
}

func (db *Database) moderatePost(boardID int, postID int, moderated PostModerated) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET moderated = $1 WHERE board = $2 AND id = $3", moderated, boardID, postID)
	if err != nil {
		log.Fatalf("failed to moderate post: %s", err)
	}
}

func (db *Database) deletePost(postID int) {
	if postID <= 0 {
		log.Panicf("invalid postID %d", postID)
	}

	_, err := db.conn.Exec(context.Background(), "DELETE FROM post WHERE id = $1", postID)
	if err != nil {
		log.Fatalf("failed to delete post: %s", err)
	}
}

func scanPost(p *Post, row pgx.Row) error {
	var boardID int
	var parentID *int
	err := row.Scan(
		&p.ID,
		&boardID,
		&parentID,
		&p.Timestamp,
		&p.Bumped,
		&p.IP,
		&p.Name,
		&p.Tripcode,
		&p.Email,
		&p.NameBlock,
		&p.Subject,
		&p.Message,
		&p.Password,
		&p.File,
		&p.FileHash,
		&p.FileOriginal,
		&p.FileSize,
		&p.FileWidth,
		&p.FileHeight,
		&p.Thumb,
		&p.ThumbWidth,
		&p.ThumbHeight,
		&p.Moderated,
		&p.Stickied,
		&p.Locked,
	)
	if err != nil {
		return err
	} else if parentID != nil {
		p.Parent = *parentID
	}
	return nil
}
