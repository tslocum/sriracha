package database

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strconv"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

const postColumns = "post.id, post.parent, post.board, post.timestamp, post.bumped, post.ip, post.name, post.tripcode, post.email, post.nameblock, post.subject, post.message, post.password, post.file, post.filehash, post.fileoriginal, post.filesize, post.filewidth, post.fileheight, post.thumb, post.thumbwidth, post.thumbheight, post.moderated, post.stickied, post.locked, post.filemime, post.backlinks"

func (db *DB) AddPost(p *Post) {
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
	err := db.conn.QueryRow(context.Background(), "INSERT INTO post VALUES (DEFAULT, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, DEFAULT, to_tsvector($26)) RETURNING id",
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
		p.FileMIME,
		p.SearchText(),
	).Scan(&p.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert post: %w", err))
	}
}

// AllThreads returns all thread IDs and reply counts. When board is nil, only
// threads belonging to boards included in the overboard are returned.
func (db *DB) AllThreads(moderated bool, board ...*Board) [][2]int {
	var boardWhere string
	l := len(board)
	if l > 0 && board[0] != nil {
		if l == 1 {
			boardWhere = fmt.Sprintf("post.board = %d AND ", board[0].ID)
		} else {
			boardWhere = "post.board IN ("
			for i, b := range board {
				if i != 0 {
					boardWhere += ","
				}
				boardWhere += strconv.Itoa(b.ID)
			}
			boardWhere += ") AND "
		}
	} else {
		var ids []byte
		for _, b := range db.AllBoards() {
			if b.Hide == HideOverboard || b.Hide == HideEverywhere {
				continue
			} else if ids != nil {
				ids = append(ids, ',')
			}
			ids = append(ids, []byte(strconv.Itoa(b.ID))...)
		}
		if len(ids) == 0 {
			return nil
		}
		boardWhere = fmt.Sprintf("post.board IN (%s) AND post.stickied = 0 AND", ids)
	}

	var extraJoin string
	var extraWhere string
	if moderated {
		extraJoin = " AND reply.moderated > 0"
		extraWhere = " AND post.moderated > 0"
	}
	rows, err := db.conn.Query(context.Background(), "SELECT post.id, COUNT(reply.id) as replies FROM post LEFT OUTER JOIN post reply ON reply.parent = post.id"+extraJoin+" WHERE "+boardWhere+" post.parent IS NULL"+extraWhere+" GROUP BY post.id ORDER BY post.stickied DESC, post.bumped DESC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select all threads: %w", err))
	}
	var threads [][2]int
	for rows.Next() {
		var thread [2]int
		err = rows.Scan(&thread[0], &thread[1])
		if err != nil {
			dbErr(err)
		}
		threads = append(threads, thread)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all threads: %w", rows.Err()))
	}
	return threads
}

func (db *DB) TrimThreads(board *Board) []*Post {
	if board.MaxThreads == 0 {
		return nil
	}
	rows, err := db.conn.Query(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE post.board = $1 AND parent IS NULL AND moderated > 0 ORDER BY bumped DESC, id ASC OFFSET $2", board.ID, board.MaxThreads)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to trim threads: %w", err))
	}
	var posts []*Post
	for rows.Next() {
		p := &Post{}
		_, err := scanPost(p, rows)
		if err != nil {
			dbErr(err)
		}
		p.Board = board
		posts = append(posts, p)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to trim threads: %w", rows.Err()))
	}
	return posts
}

func (db *DB) AllPostsInThread(moderated bool, postID int) []*Post {
	var extra string
	if moderated {
		extra = " AND moderated > 0"
	}
	rows, err := db.conn.Query(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE (id = $1 OR parent = $1)"+extra+" ORDER BY id ASC", postID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to select all posts in thread %d: %s", postID, err))
	}
	var posts []*Post
	var boardIDs []int
	for rows.Next() {
		p := &Post{}
		boardID, err := scanPost(p, rows)
		if err != nil {
			dbErr(err)
		}
		posts = append(posts, p)
		boardIDs = append(boardIDs, boardID)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all pages: %w", rows.Err()))
	}
	for i := range posts {
		posts[i].Board = db.BoardByID(boardIDs[i])
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all posts in thread %d: %s", postID, rows.Err()))
	}
	return posts
}

func (db *DB) AllReplies(threadID int, limit int, moderated bool) []*Post {
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
	rows, err := db.conn.Query(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE parent = $1"+extraModerated+" ORDER BY id "+sortDir+extraLimit, threadID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to select all replies: %w", err))
	}
	var posts []*Post
	var boardIDs []int
	for rows.Next() {
		p := &Post{}
		boardID, err := scanPost(p, rows)
		if err != nil {
			dbErr(err)
		}
		posts = append(posts, p)
		boardIDs = append(boardIDs, boardID)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all replies: %w", rows.Err()))
	}
	for i := range posts {
		posts[i].Board = db.BoardByID(boardIDs[i])
	}
	if sortDir == "DESC" {
		slices.Reverse(posts)
	}
	return posts
}

func (db *DB) PendingPosts() []*Post {
	rows, err := db.conn.Query(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE moderated = $1 ORDER BY id ASC", ModeratedHidden)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to select pending posts: %w", err))
	}
	var posts []*Post
	var boardIDs []int
	for rows.Next() {
		p := &Post{}
		boardID, err := scanPost(p, rows)
		if err != nil {
			dbErr(err)
		}
		posts = append(posts, p)
		boardIDs = append(boardIDs, boardID)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select pending posts: %w", rows.Err()))
	}
	for i := range posts {
		posts[i].Board = db.BoardByID(boardIDs[i])
	}
	return posts
}

func (db *DB) PostByID(postID int) *Post {
	p := &Post{}
	boardID, err := scanPost(p, db.conn.QueryRow(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE id = $1", postID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to select post: %w", err))
	}
	p.Board = db.BoardByID(boardID)
	return p
}

func (db *DB) PostsByIP(hash string) []*Post {
	if hash == "" {
		return nil
	}
	rows, err := db.conn.Query(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE ip = $1", hash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to select post: %w", err))
	}
	var posts []*Post
	var boardIDs []int
	for rows.Next() {
		p := &Post{}
		boardID, err := scanPost(p, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to scan post: %w", err))
		}
		posts = append(posts, p)
		boardIDs = append(boardIDs, boardID)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select post: %w", rows.Err()))
	}
	for i, p := range posts {
		p.Board = db.BoardByID(boardIDs[i])
	}
	return posts
}

func (db *DB) PostsByFileHash(hash string, filterBoard *Board) []*Post {
	var extra string
	if filterBoard != nil {
		extra = " AND post.board = " + strconv.Itoa(filterBoard.ID)
	}
	rows, err := db.conn.Query(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE filehash = $1 "+extra, hash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to select post: %w", err))
	}
	var posts []*Post
	var boardIDs []int
	for rows.Next() {
		p := &Post{}
		boardID, err := scanPost(p, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to scan post: %w", err))
		}
		posts = append(posts, p)
		boardIDs = append(boardIDs, boardID)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select post: %w", rows.Err()))
	}
	for i, p := range posts {
		p.Board = db.BoardByID(boardIDs[i])
	}
	return posts
}

func (db *DB) PostsByBacklink(targetID int) []*Post {
	rows, err := db.conn.Query(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE $1 = ANY(backlinks)", targetID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to select posts: %w", err))
	}
	var posts []*Post
	var boardIDs []int
	for rows.Next() {
		p := &Post{}
		boardID, err := scanPost(p, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to scan post: %w", err))
		}
		posts = append(posts, p)
		boardIDs = append(boardIDs, boardID)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select posts: %w", rows.Err()))
	}
	for i, p := range posts {
		p.Board = db.BoardByID(boardIDs[i])
	}
	return posts
}

func (db *DB) PostByField(b *Board, field string, value any) *Post {
	p := &Post{}
	_, err := scanPost(p, db.conn.QueryRow(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE post.board = $1 AND "+field+" = $2 LIMIT 1", b.ID, value))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to select post: %w", err))
	} else if p.ID == 0 {
		return nil
	}
	p.Board = b
	return p
}

func (db *DB) LastPostByIP(board *Board, ip string) *Post {
	p := &Post{}
	boardID, err := scanPost(p, db.conn.QueryRow(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE post.board = $1 AND ip = $2 ORDER BY id DESC LIMIT 1", board.ID, ip))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select last post by IP: %w", err))
	}
	p.Board = db.BoardByID(boardID)
	return p
}

func (db *DB) LastPostByBoard(board *Board) *Post {
	p := &Post{}
	_, err := scanPost(p, db.conn.QueryRow(context.Background(), "SELECT "+postColumns+", 0 as replies FROM post WHERE post.board = $1 AND moderated> 0 ORDER BY id DESC LIMIT 1", board.ID))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select last post by board: %w", err))
	}
	p.Board = board
	return p
}

func (db *DB) SearchPosts(query string, board ...*Board) []int {
	var extra string
	if len(board) > 0 && board[0] != nil {
		extra = "board IN ("
		for i, b := range board {
			if i != 0 {
				extra += ","
			}
			extra += strconv.Itoa(b.ID)
		}
		extra += ") AND "
	}
	rows, err := db.conn.Query(context.Background(), "SELECT id, ts_rank_cd(search, query) AS rank FROM post, websearch_to_tsquery($1) AS query WHERE "+extra+"query @@ search AND moderated > 0 ORDER BY rank DESC", query)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to search posts: %w", err))
	}
	var postIDs []int
	for rows.Next() {
		var postID int
		var rank float64
		err = rows.Scan(&postID, &rank)
		if err != nil {
			dbErr(fmt.Errorf("failed to scan post: %w", err))
		}
		postIDs = append(postIDs, postID)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to search posts: %w", rows.Err()))
	}
	return postIDs
}

func (db *DB) NumPosts(filterBoard *Board, since int64) int {
	var extraWhere string
	if filterBoard != nil {
		extraWhere += fmt.Sprintf("board = %d AND ", filterBoard.ID)
	}
	if since > 0 {
		extraWhere += fmt.Sprintf("timestamp >= %d AND ", since)
	}
	var count int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM post WHERE "+extraWhere+"moderated > 0").Scan(&count)
	if err == pgx.ErrNoRows {
		return 0
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select number of posts: %w", err))
	}
	return count
}

func (db *DB) ReplyCount(threadID int) int {
	var count int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM post WHERE parent = $1", threadID).Scan(&count)
	if err == pgx.ErrNoRows {
		return 0
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select reply count: %w", err))
	}
	return count
}

func (db *DB) MaxPostID() int {
	var id int
	err := db.conn.QueryRow(context.Background(), "SELECT id FROM post ORDER BY id DESC LIMIT 1").Scan(&id)
	if err == pgx.ErrNoRows {
		return 0
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select maximum post ID: %w", err))
	}
	return id
}

func (db *DB) BumpThread(threadID int, timestamp int64) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET bumped = $1 WHERE id = $2 AND bumped < $1", timestamp, threadID)
	if err != nil {
		dbErr(fmt.Errorf("failed to bump thread: %w", err))
	}
}

func (db *DB) ModeratePost(postID int, moderated PostModerated) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET moderated = $1 WHERE id = $2", moderated, postID)
	if err != nil {
		dbErr(fmt.Errorf("failed to moderate post: %w", err))
	}
}

func (db *DB) StickyPost(postID int, sticky bool) {
	var stickied int
	if sticky {
		stickied = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET stickied = $1 WHERE id = $2", stickied, postID)
	if err != nil {
		dbErr(fmt.Errorf("failed to sticky post: %w", err))
	}
}

func (db *DB) LockPost(postID int, lock bool) {
	var locked int
	if lock {
		locked = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET locked = $1 WHERE id = $2", locked, postID)
	if err != nil {
		dbErr(fmt.Errorf("failed to lock post: %w", err))
	}
}

func (db *DB) UpdatePostBoard(postID int, board *Board) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET board = $1 WHERE id = $2", board.ID, postID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update post board: %w", err))
	}
}

func (db *DB) UpdatePostNameblock(postID int, nameblock string) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET nameblock = $1 WHERE id = $2", nameblock, postID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update post nameblock: %w", err))
	}
}

func (db *DB) UpdatePostMessage(postID int, message string) {
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET message = $1 WHERE id = $2", message, postID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update post message: %w", err))
	}
}

func (db *DB) DeletePost(postID int) {
	if postID <= 0 {
		log.Panicf("invalid post ID %d", postID)
	}

	_, err := db.conn.Exec(context.Background(), "DELETE FROM post WHERE id = $1", postID)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete post: %w", err))
	}
	for _, p := range db.PostsByBacklink(postID) {
		i := slices.Index(p.Backlinks, postID)
		if i == -1 {
			continue
		}
		p.Backlinks = append(p.Backlinks[:i], p.Backlinks[i+1:]...)
		_, err = db.conn.Exec(context.Background(), "UPDATE post SET backlinks = $1 WHERE id = $2", p.Backlinks, p.ID)
		if err != nil {
			dbErr(fmt.Errorf("failed to update post backlinks: %w", err))
		}
	}
	db.DeleteSubscriptionsByPost(postID)
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
		&p.FileMIME,
		&p.Backlinks,
		// Replies are selected as a separate value, so they come after post fields.
		&p.Replies,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, err
		}
		return 0, fmt.Errorf("failed to scan post: %w", err)
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

func (db *DB) AddPostBacklink(target *Post, sourceID int) {
	if slices.Contains(target.Backlinks, sourceID) {
		return
	}
	target.Backlinks = append(target.Backlinks, sourceID)
	slices.Sort(target.Backlinks)
	_, err := db.conn.Exec(context.Background(), "UPDATE post SET backlinks = $1 WHERE id = $2", target.Backlinks, target.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update post backlinks: %w", err))
	}
}

func (db *DB) AddPostBacklinks(p *Post) {
	for _, mention := range p.Mentions() {
		mentionPost := db.PostByID(mention)
		if mentionPost == nil || mentionPost.Thread() != p.Thread() {
			continue
		}
		db.AddPostBacklink(mentionPost, p.ID)
	}
}

func (db *DB) HavePostBacklinks() bool {
	var haveBacklinks bool
	err := db.conn.QueryRow(context.Background(), "SELECT EXISTS (SELECT * FROM post WHERE  array_length(backlinks, 1) != 0 LIMIT 1)").Scan(&haveBacklinks)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert post: %w", err))
	}
	return haveBacklinks
}
