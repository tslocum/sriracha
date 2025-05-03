package sriracha

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addPost(p *Post) {
	var parent *int
	if p.Parent != 0 {
		parent = &p.Parent
	}
	var fileHash *string
	if p.FileHash != "" {
		fileHash = &p.FileHash
	}
	var stickied int
	if p.Stickied {
		stickied = 1
	}
	var locked int
	if p.Locked {
		locked = 1
	}
	err := db.conn.QueryRow(context.Background(), "INSERT INTO post VALUES (DEFAULT, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24) RETURNING id",
		parent,
		p.Board.ID,
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
		fileHash,
		p.FileOriginal,
		p.FileSize,
		p.FileWidth,
		p.FileHeight,
		p.Thumb,
		p.ThumbWidth,
		p.ThumbHeight,
		p.Moderated,
		stickied,
		locked,
	).Scan(&p.ID)
	if err != nil || p.ID == 0 {
		log.Fatalf("failed to insert post: %s", err)
	}
}

func (db *Database) allThreads(board *Board, moderated bool) []*Post {
	var extraJoin string
	var extraWhere string
	if moderated {
		extraJoin = " AND reply.moderated > 0"
		extraWhere = " AND post.moderated > 0"
	}
	rows, err := db.conn.Query(context.Background(), "SELECT post.*, COUNT(reply.id) as replies FROM post LEFT OUTER JOIN post reply ON reply.parent = post.id"+extraJoin+" WHERE post.board = $1 AND post.parent IS NULL"+extraWhere+" GROUP BY post.id ORDER BY stickied DESC, bumped DESC", board.ID)
	if err != nil {
		log.Fatalf("failed to select all posts: %s", err)
	}
	var posts []*Post
	for rows.Next() {
		p := &Post{}
		_, err := scanPost(p, rows)
		if err != nil {
			log.Fatal(err)
		}
		p.Board = board
		posts = append(posts, p)
	}
	return posts
}

func (db *Database) trimThreads(board *Board) []*Post {
	if board.MaxThreads == 0 {
		return nil
	}
	rows, err := db.conn.Query(context.Background(), "SELECT *, 0 as replies FROM post WHERE board = $1 AND parent IS NULL AND moderated > 0 ORDER BY bumped DESC OFFSET $2", board.ID, board.MaxThreads)
	if err != nil {
		log.Fatalf("failed to select trim threads: %s", err)
	}
	var posts []*Post
	for rows.Next() {
		p := &Post{}
		_, err := scanPost(p, rows)
		if err != nil {
			log.Fatal(err)
		}
		p.Board = board
		posts = append(posts, p)
	}
	return posts
}

func (db *Database) allPostsInThread(postID int, moderated bool) []*Post {
	var extra string
	if moderated {
		extra = " AND moderated > 0"
	}
	rows, err := db.conn.Query(context.Background(), "SELECT *, 0 as replies FROM post WHERE (id = $1 OR parent = $1)"+extra+" ORDER BY id ASC", postID)
	if err != nil {
		log.Fatalf("failed to select all posts: %s", err)
	}
	var posts []*Post
	var boardIDs []int
	for rows.Next() {
		p := &Post{}
		boardID, err := scanPost(p, rows)
		if err != nil {
			log.Fatal(err)
		}
		posts = append(posts, p)
		boardIDs = append(boardIDs, boardID)
	}
	for i := range posts {
		posts[i].Board = db.boardByID(boardIDs[i])
	}
	return posts
}

func (db *Database) allReplies(threadID int, limit int, moderated bool) []*Post {
	if limit == 0 {
		return nil
	}
	var sortDir = "ASC"
	var extraLimit string
	if limit != 0 {
		sortDir = "DESC"
		extraLimit = fmt.Sprintf(" LIMIT %d", limit)
	}
	var extraModerated string
	if moderated {
		extraModerated = " AND moderated > 0"
	}
	rows, err := db.conn.Query(context.Background(), "SELECT *, 0 as replies FROM post WHERE parent = $1"+extraModerated+" ORDER BY id "+sortDir+extraLimit, threadID)
	if err != nil {
		log.Fatalf("failed to select all replies: %s", err)
	}
	var posts []*Post
	var boardIDs []int
	for rows.Next() {
		p := &Post{}
		boardID, err := scanPost(p, rows)
		if err != nil {
			log.Fatal(err)
		}
		posts = append(posts, p)
		boardIDs = append(boardIDs, boardID)
	}
	for i := range posts {
		posts[i].Board = db.boardByID(boardIDs[i])
	}
	if sortDir == "DESC" {
		slices.Reverse(posts)
	}
	return posts
}

func (db *Database) pendingPosts() []*Post {
	rows, err := db.conn.Query(context.Background(), "SELECT *, 0 as replies FROM post WHERE moderated = $1 ORDER BY id ASC", ModeratedHidden)
	if err != nil {
		log.Fatalf("failed to select pending posts: %s", err)
	}
	var posts []*Post
	var boardIDs []int
	for rows.Next() {
		p := &Post{}
		boardID, err := scanPost(p, rows)
		if err != nil {
			log.Fatal(err)
		}
		posts = append(posts, p)
		boardIDs = append(boardIDs, boardID)
	}
	for i := range posts {
		posts[i].Board = db.boardByID(boardIDs[i])
	}
	return posts
}

func (db *Database) postByID(postID int) *Post {
	p := &Post{}
	boardID, err := scanPost(p, db.conn.QueryRow(context.Background(), "SELECT *, 0 as replies FROM post WHERE id = $1", postID))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil || p.ID == 0 {
		log.Fatalf("failed to select post: %s", err)
	}
	p.Board = db.boardByID(boardID)
	return p
}

func (db *Database) postByFileHash(hash string) *Post {
	p := &Post{}
	boardID, err := scanPost(p, db.conn.QueryRow(context.Background(), "SELECT *, 0 as replies FROM post WHERE filehash = $1", hash))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil || p.ID == 0 {
		log.Fatalf("failed to select post: %s", err)
	}
	p.Board = db.boardByID(boardID)
	return p
}

func (db *Database) PostByField(b *Board, field string, value any) *Post {
	p := &Post{}
	_, err := scanPost(p, db.conn.QueryRow(context.Background(), "SELECT *, 0 as replies FROM post WHERE board = $1 AND "+field+" = $2 LIMIT 1", b.ID, value))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil || p.ID == 0 {
		log.Fatalf("failed to select post: %s", err)
	}
	p.Board = b
	return p
}

func (db *Database) lastPostByIP(board *Board, ip string) *Post {
	p := &Post{}
	boardID, err := scanPost(p, db.conn.QueryRow(context.Background(), "SELECT *, 0 as replies FROM post WHERE board = $1 AND ip = $2 ORDER BY id DESC LIMIT 1", board.ID, ip))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil || p.ID == 0 {
		log.Fatalf("failed to select last post by IP: %s", err)
	}
	p.Board = db.boardByID(boardID)
	return p
}

func (db *Database) replyCount(threadID int) int {
	var count int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM post WHERE parent = $1", threadID).Scan(&count)
	if err == pgx.ErrNoRows {
		return 0
	} else if err != nil {
		log.Fatalf("failed to select reply count: %s", err)
	}
	return count
}

func (db *Database) bumpThread(threadID int, timestamp int64) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET bumped = $1 WHERE id = $2 AND bumped < $1", timestamp, threadID)
	if err != nil {
		log.Fatalf("failed to bump thread: %s", err)
	}
}

func (db *Database) moderatePost(postID int, moderated PostModerated) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET moderated = $1 WHERE id = $2", moderated, postID)
	if err != nil {
		log.Fatalf("failed to moderate post: %s", err)
	}
}

func (db *Database) stickyPost(postID int, sticky bool) {
	var stickied int
	if sticky {
		stickied = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET stickied = $1 WHERE id = $2", stickied, postID)
	if err != nil {
		log.Fatalf("failed to sticky post: %s", err)
	}
}

func (db *Database) lockPost(postID int, lock bool) {
	var locked int
	if lock {
		locked = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET locked = $1 WHERE id = $2", locked, postID)
	if err != nil {
		log.Fatalf("failed to lock post: %s", err)
	}
}

func (db *Database) updatePostMessage(postID int, message string) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET message = $1 WHERE id = $2", message, postID)
	if err != nil {
		log.Fatalf("failed to update post message: %s", err)
	}
}

func (db *Database) deletePost(postID int) {
	if postID <= 0 {
		log.Panicf("invalid post ID %d", postID)
	}

	_, err := db.conn.Exec(context.Background(), "DELETE FROM post WHERE id = $1", postID)
	if err != nil {
		log.Fatalf("failed to delete post: %s", err)
	}
}

func scanPost(p *Post, row pgx.Row) (int, error) {
	var (
		parentID *int
		boardID  int
		fileHash *string
		stickied int
		locked   int
	)
	err := row.Scan(
		&p.ID,
		&parentID,
		&boardID,
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
		&fileHash,
		&p.FileOriginal,
		&p.FileSize,
		&p.FileWidth,
		&p.FileHeight,
		&p.Thumb,
		&p.ThumbWidth,
		&p.ThumbHeight,
		&p.Moderated,
		&stickied,
		&locked,
		&p.Replies,
	)
	if err != nil {
		return 0, err
	}
	if parentID != nil {
		p.Parent = *parentID
	}
	if fileHash != nil {
		p.FileHash = *fileHash
	}
	p.Stickied = stickied == 1
	p.Locked = locked == 1
	return boardID, nil
}
