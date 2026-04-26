package database

import (
	"context"
	"fmt"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) fetchBannerBoards(b *Banner) {
	b.Boards = nil

	rows, err := db.conn.Query(context.Background(), "SELECT board FROM banner_board WHERE banner = $1", b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to select banner boards: %w", err))
	}
	var ids []int
	for rows.Next() {
		var id int
		err := rows.Scan(&id)
		if err != nil {
			dbErr(fmt.Errorf("failed to select banner boards: %w", err))
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select banner boards: %w", rows.Err()))
	}

	for _, id := range ids {
		board := db.BoardByID(id)
		b.Boards = append(b.Boards, board)
	}
}

func (db *DB) updateBannerBoards(b *Banner) {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM banner_board WHERE banner = $1", b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update banner boards: %w", err))
	}
	for _, board := range b.Boards {
		_, err = db.conn.Exec(context.Background(), "INSERT INTO banner_board VALUES ($1, $2)", b.ID, board.ID)
		if err != nil {
			dbErr(fmt.Errorf("failed to update banner boards: %w", err))
		}
	}
}

func (db *DB) AddBanner(b *Banner) {
	var overboard int
	if b.Overboard {
		overboard = 1
	}
	var news int
	if b.News {
		news = 1
	}
	var pages int
	if b.Pages {
		pages = 1
	}
	err := db.conn.QueryRow(context.Background(), "INSERT INTO banner VALUES (DEFAULT, $1, $2, $3, $4, $5, $6) RETURNING id",
		b.Name,
		b.Width,
		b.Height,
		overboard,
		news,
		pages,
	).Scan(&b.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert banner: %w", err))
	}
	db.updateBannerBoards(b)
}

func (db *DB) BannerByID(id int) *Banner {
	b := &Banner{}
	err := scanBanner(b, db.conn.QueryRow(context.Background(), "SELECT * FROM banner WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select banner: %w", err))
	}
	db.fetchBannerBoards(b)
	return b
}

func (db *DB) BannerByName(name string) *Banner {
	b := &Banner{}
	err := scanBanner(b, db.conn.QueryRow(context.Background(), "SELECT * FROM banner WHERE name = $1", name))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select banner: %w", err))
	}
	db.fetchBannerBoards(b)
	return b
}

func (db *DB) AllBanners() []*Banner {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM banner ORDER BY name ASC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select all banners: %w", err))
	}
	var banners []*Banner
	for rows.Next() {
		b := &Banner{}
		err := scanBanner(b, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select all banners: %w", err))
		}
		banners = append(banners, b)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select all banners: %w", rows.Err()))
	}
	for _, b := range banners {
		db.fetchBannerBoards(b)
	}
	return banners
}

func (db *DB) UpdateBanner(b *Banner) {
	if b.ID <= 0 {
		dbErr(fmt.Errorf("invalid banner ID %d", b.ID))
	}
	var overboard int
	if b.Overboard {
		overboard = 1
	}
	var news int
	if b.News {
		news = 1
	}
	var pages int
	if b.Pages {
		pages = 1
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE banner SET name = $1, width = $2, height = $3, overboard = $4, news = $5, pages = $6 WHERE id = $7",
		b.Name,
		b.Width,
		b.Height,
		overboard,
		news,
		pages,
		b.ID,
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to update banner: %w", err))
	}
	db.updateBannerBoards(b)
}

func (db *DB) DeleteBanner(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM banner WHERE id = $1", id)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete banner: %w", err))
	}
}

func scanBanner(b *Banner, row pgx.Row) error {
	var overboard, news, pages int
	err := row.Scan(
		&b.ID,
		&b.Name,
		&b.Width,
		&b.Height,
		&overboard,
		&news,
		&pages,
	)
	if err != nil {
		return err
	}
	b.Overboard = overboard == 1
	b.News = news == 1
	b.Pages = pages == 1
	return nil
}
