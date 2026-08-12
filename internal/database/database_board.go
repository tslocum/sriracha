package database

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

// Cache boards to reduce pressure on the database.
var boardCache []*Board
var boardCacheID = make(map[int]*Board)
var boardCacheDir = make(map[string]*Board)
var boardCacheLock = &sync.RWMutex{}

func (db *DB) setBoardAttributes(b *Board) {
	rows, err := db.conn.Query(context.Background(), "SELECT upload FROM board_upload WHERE board = $1", b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to select board uploads: %w", err))
	}
	b.Uploads = nil
	for rows.Next() {
		var mimeType string
		err := rows.Scan(&mimeType)
		if err != nil {
			dbErr(fmt.Errorf("failed to select board uploads: %w", err))
		}
		for _, u := range db.config.UploadTypes() {
			if u.MIME == mimeType {
				b.Uploads = append(b.Uploads, u.MIME)
				break
			}
		}
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select board uploads: %w", rows.Err()))
	}

	rows, err = db.conn.Query(context.Background(), "SELECT embed FROM board_embed WHERE board = $1", b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to select board embeds: %w", err))
	}
	b.Embeds = nil
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			dbErr(fmt.Errorf("failed to select board embeds: %w", err))
		}
		b.Embeds = append(b.Embeds, name)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select board embeds: %w", rows.Err()))
	}
}

func (db *DB) AddBoard(b *Board) {
	db.ClearBoardCache()

	var reports int
	if b.Reports {
		reports = 1
	}
	var oekaki int
	if b.Oekaki {
		oekaki = 1
	}
	var backlinks int
	if b.Backlinks {
		backlinks = 1
	}
	var gallery int
	if b.Gallery {
		gallery = 1
	}
	err := db.conn.QueryRow(context.Background(), "INSERT INTO board VALUES (DEFAULT, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40) RETURNING id",
		b.Dir,
		b.Name,
		b.Description,
		b.Type,
		b.Lock,
		b.Approval,
		reports,
		b.Style,
		b.Locale,
		b.MinName,
		b.MaxName,
		b.MinEmail,
		b.MaxEmail,
		b.MinSubject,
		b.MaxSubject,
		b.MinMessage,
		b.MaxMessage,
		b.MinSizeThread,
		b.MaxSizeThread,
		b.MinSizeReply,
		b.MaxSizeReply,
		b.ThumbWidth,
		b.ThumbHeight,
		b.DefaultName,
		b.WordBreak,
		b.Truncate,
		b.Threads,
		b.Replies,
		b.MaxThreads,
		b.MaxReplies,
		oekaki,
		strings.Join(b.Rules, sriracha.Divider),
		b.Hide,
		backlinks,
		b.Instances,
		b.Identifiers,
		b.Files,
		gallery,
		b.Require,
		b.Archive,
	).Scan(&b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert board: %w", err))
	}
	for _, upload := range b.Uploads {
		_, err := db.conn.Exec(context.Background(), "INSERT INTO board_upload VALUES ($1, $2)", b.ID, upload)
		if err != nil {
			dbErr(fmt.Errorf("failed to insert board uploads: %w", err))
		}
	}
	for _, embed := range b.Embeds {
		_, err := db.conn.Exec(context.Background(), "INSERT INTO board_embed VALUES ($1, $2)", b.ID, embed)
		if err != nil {
			dbErr(fmt.Errorf("failed to insert board embeds: %w", err))
		}
	}
}

func (db *DB) AllBoards() []*Board {
	boardCacheLock.RLock()
	if boardCache != nil {
		boardCacheLock.RUnlock()
		return boardCache
	}
	boardCacheLock.RUnlock()
	boardCacheLock.Lock()

	rows, err := db.conn.Query(context.Background(), "SELECT * FROM board ORDER BY dir ASC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select all boards: %w", err))
	}
	for rows.Next() {
		b := &Board{}
		err := scanBoard(b, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select all boards: %w", err))
		}
		boardCache = append(boardCache, b)
		boardCacheID[b.ID] = b
		boardCacheDir[b.Dir] = b
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all boards: %w", rows.Err()))
	}
	for _, b := range boardCache {
		db.setBoardAttributes(b)
	}

	boardCacheLock.Unlock()
	return boardCache
}

func (db *DB) BoardByID(id int) *Board {
	boardCacheLock.RLock()
	b := boardCacheID[id]
	if b != nil {
		boardCacheLock.RUnlock()
		return b
	}
	boardCacheLock.RUnlock()
	for _, b := range db.AllBoards() {
		if b.ID == id {
			return b
		}
	}
	return nil
}

func (db *DB) BoardByDir(dir string) *Board {
	boardCacheLock.RLock()
	b := boardCacheDir[dir]
	if b != nil {
		boardCacheLock.RUnlock()
		return b
	}
	boardCacheLock.RUnlock()
	for _, b := range db.AllBoards() {
		if b.Dir == dir {
			return b
		}
	}
	return nil
}

func (db *DB) UniqueUserPosts(b *Board) int {
	var count int
	var err error
	if b == nil {
		err = db.conn.QueryRow(context.Background(), "SELECT COUNT(DISTINCT ip) FROM post").Scan(&count)
	} else {
		err = db.conn.QueryRow(context.Background(), "SELECT COUNT(DISTINCT ip) FROM post WHERE board = $1", b.ID).Scan(&count)
	}
	if err == pgx.ErrNoRows {
		return 0
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select unique user posts: %w", err))
	}
	return count
}

func (db *DB) UpdateBoard(b *Board) {
	if b.ID <= 0 {
		dbErr(fmt.Errorf("invalid board ID %d", b.ID))
	}
	db.ClearBoardCache()

	var reports int
	if b.Reports {
		reports = 1
	}
	var oekaki int
	if b.Oekaki {
		oekaki = 1
	}
	var backlinks int
	if b.Backlinks {
		backlinks = 1
	}
	var gallery int
	if b.Gallery {
		gallery = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE board SET dir = $1, name = $2, description = $3, type = $4, lock = $5, approval = $6, reports = $7, style = $8, locale = $9, minname = $10, maxname = $11, minemail = $12, maxemail = $13, minsubject = $14, maxsubject = $15, minmessage = $16, maxmessage = $17, minsizethread = $18, maxsizethread = $19, minsizereply = $20, maxsizereply = $21, thumbwidth = $22, thumbheight = $23, defaultname = $24, wordbreak = $25, truncate = $26, threads = $27, replies = $28, maxthreads = $29, maxreplies = $30, oekaki = $31, rules = $32, hide = $33, backlinks = $34, instances = $35, identifiers = $36, files = $37, gallery = $38, require = $39, archive = $40 WHERE id = $41",
		b.Dir,
		b.Name,
		b.Description,
		b.Type,
		b.Lock,
		b.Approval,
		reports,
		b.Style,
		b.Locale,
		b.MinName,
		b.MaxName,
		b.MinEmail,
		b.MaxEmail,
		b.MinSubject,
		b.MaxSubject,
		b.MinMessage,
		b.MaxMessage,
		b.MinSizeThread,
		b.MaxSizeThread,
		b.MinSizeReply,
		b.MaxSizeReply,
		b.ThumbWidth,
		b.ThumbHeight,
		b.DefaultName,
		b.WordBreak,
		b.Truncate,
		b.Threads,
		b.Replies,
		b.MaxThreads,
		b.MaxReplies,
		oekaki,
		strings.Join(b.Rules, sriracha.Divider),
		b.Hide,
		backlinks,
		b.Instances,
		b.Identifiers,
		b.Files,
		gallery,
		b.Require,
		b.Archive,
		b.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update board: %w", err))
	}

	_, err = db.conn.Exec(context.Background(), "DELETE FROM board_upload WHERE board = $1", b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete board uploads: %w", err))
	}
	for _, upload := range b.Uploads {
		_, err := db.conn.Exec(context.Background(), "INSERT INTO board_upload VALUES ($1, $2)", b.ID, upload)
		if err != nil {
			dbErr(fmt.Errorf("failed to insert board uploads: %w", err))
		}
	}

	_, err = db.conn.Exec(context.Background(), "DELETE FROM board_embed WHERE board = $1", b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete board embeds: %w", err))
	}
	for _, embed := range b.Embeds {
		_, err := db.conn.Exec(context.Background(), "INSERT INTO board_embed VALUES ($1, $2)", b.ID, embed)
		if err != nil {
			dbErr(fmt.Errorf("failed to insert board embeds: %w", err))
		}
	}
}

func (db *DB) DeleteBoard(id int) {
	if id == 0 {
		return
	}
	db.ClearBoardCache()

	_, err := db.conn.Exec(context.Background(), "DELETE FROM board WHERE id = $1", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete board: %w", err))
	}
	db.DeleteSubscriptionsByBoard(id)
}

func (db *DB) ClearBoardCache() {
	boardCache = nil
	clear(boardCacheID)
	clear(boardCacheDir)
}

func scanBoard(b *Board, row pgx.Row) error {
	var (
		reports   int
		oekaki    int
		rules     string
		backlinks int
		gallery   int
	)
	err := row.Scan(
		&b.ID,
		&b.Dir,
		&b.Name,
		&b.Description,
		&b.Type,
		&b.Lock,
		&b.Approval,
		&reports,
		&b.Style,
		&b.Locale,
		&b.MinName,
		&b.MaxName,
		&b.MinEmail,
		&b.MaxEmail,
		&b.MinSubject,
		&b.MaxSubject,
		&b.MinMessage,
		&b.MaxMessage,
		&b.MinSizeThread,
		&b.MaxSizeThread,
		&b.MinSizeReply,
		&b.MaxSizeReply,
		&b.ThumbWidth,
		&b.ThumbHeight,
		&b.DefaultName,
		&b.WordBreak,
		&b.Truncate,
		&b.Threads,
		&b.Replies,
		&b.MaxThreads,
		&b.MaxReplies,
		&oekaki,
		&rules,
		&b.Hide,
		&backlinks,
		&b.Instances,
		&b.Identifiers,
		&b.Files,
		&gallery,
		&b.Require,
		&b.Archive,
	)
	if err != nil {
		return err
	}
	b.Reports = reports == 1
	b.Oekaki = oekaki == 1
	if rules != "" {
		b.Rules = strings.Split(rules, sriracha.Divider)
	}
	b.Backlinks = backlinks == 1
	b.Gallery = gallery == 1
	return nil
}
