package database

import (
	"context"
	"log"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) fetchBannerBoards(b *Banner) {
	b.Boards = nil

	rows, err := db.conn.Query(context.Background(), "SELECT board FROM banner_board WHERE banner = $1", b.ID)
	if err != nil {
		log.Fatalf("failed to select banner boards: %s", err)
	}
	var ids []int
	for rows.Next() {
		var id int
		err := rows.Scan(&id)
		if err != nil {
			log.Fatalf("failed to select banner boards: %s", err)
		}
		ids = append(ids, id)
	}

	for _, id := range ids {
		board := db.BoardByID(id)
		b.Boards = append(b.Boards, board)
	}
}

func (db *DB) updateBannerBoards(b *Banner) {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM banner_board WHERE banner = $1", b.ID)
	if err != nil {
		log.Fatalf("failed to update banner boards: %s", err)
	}
	for _, board := range b.Boards {
		_, err = db.conn.Exec(context.Background(), "INSERT INTO banner_board VALUES ($1, $2)", b.ID, board.ID)
		if err != nil {
			log.Fatalf("failed to update banner boards: %s", err)
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
	_, err := db.conn.Exec(context.Background(), "INSERT INTO banner VALUES (DEFAULT, $1, $2, $3, $4, $5, $6)",
		b.Name,
		b.Width,
		b.Height,
		overboard,
		news,
		pages,
	)
	if err != nil {
		log.Fatalf("failed to insert banner: %s", err)
	}
	err = db.conn.QueryRow(context.Background(), "SELECT id FROM banner WHERE name = $1", b.Name).Scan(&b.ID)
	if err != nil {
		log.Fatalf("failed to select id of added banner: %s", err)
	} else if b.ID == 0 {
		log.Fatal("failed to select id of added banner")
	}
	db.updateBannerBoards(b)
}

func (db *DB) BannerByID(id int) *Banner {
	b := &Banner{}
	err := scanBanner(b, db.conn.QueryRow(context.Background(), "SELECT * FROM banner WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select banner: %s", err)
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
		log.Fatalf("failed to select banner: %s", err)
	}
	db.fetchBannerBoards(b)
	return b
}

func (db *DB) AllBanners() []*Banner {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM banner ORDER BY name ASC")
	if err != nil {
		log.Fatalf("failed to select all banners: %s", err)
	}
	var banners []*Banner
	for rows.Next() {
		b := &Banner{}
		err := scanBanner(b, rows)
		if err != nil {
			log.Fatalf("failed to select all banners: %s", err)
		}
		banners = append(banners, b)
	}
	for _, b := range banners {
		db.fetchBannerBoards(b)
	}
	return banners
}

func (db *DB) UpdateBanner(b *Banner) {
	if b.ID <= 0 {
		log.Fatalf("invalid banner ID %d", b.ID)
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
		log.Fatalf("failed to update banner: %s", err)
	}
	db.updateBannerBoards(b)
}

func (db *DB) DeleteBanner(id int) {
	if id == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM banner WHERE id = $1", id)
	if err != nil {
		log.Fatalf("failed to delete banner: %s", err)
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
