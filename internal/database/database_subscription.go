package database

import (
	"context"
	"log"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddSubscription(s *Subscription) {
	err := db.conn.QueryRow(context.Background(), "INSERT INTO subscription VALUES (DEFAULT, $1, $2, $3, $4, $5) RETURNING id",
		s.IP,
		s.Confirm,
		s.Email,
		s.Board,
		s.Target).Scan(&s.ID)
	if err != nil {
		log.Fatalf("failed to insert subscription: %s", err)
	}
}

func (db *DB) SubscriptionByID(id int) *Subscription {
	s := &Subscription{}
	err := scanSubscription(s, db.conn.QueryRow(context.Background(), "SELECT * FROM subscription WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select subscription: %s", err)
	}
	return s
}

func (db *DB) SubscriptionsByEmail(email string) []*Subscription {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM subscription WHERE email = $1 ORDER BY board DESC, target ASC", email)
	if err != nil {
		log.Fatalf("failed to select subscriptions: %s", err)
	}
	var subs []*Subscription
	for rows.Next() {
		s := &Subscription{}
		err := scanSubscription(s, rows)
		if err != nil {
			log.Fatalf("failed to select subscriptions: %s", err)
		}
		subs = append(subs, s)
	}
	return subs
}

func (db *DB) SubscriptionsByPost(p *Post, distinct bool, includeBoard bool) []*Subscription {
	query := "SELECT"
	if distinct {
		query = "SELECT DISTINCT ON (email)"
	}
	query += " * FROM subscription WHERE (target = $1"
	args := []any{p.ID}
	if includeBoard {
		if p.Parent == 0 {
			query += " OR board = $2"
		} else {
			query += " OR (board = $2 AND target = 0)"
		}
		args = append(args, p.Board.ID)
	}
	query += ") AND confirm = 0 ORDER BY email ASC, target DESC, board ASC"
	rows, err := db.conn.Query(context.Background(), query, args...)
	if err != nil {
		log.Fatalf("failed to select subscriptions: %s", err)
	}
	var subs []*Subscription
	for rows.Next() {
		s := &Subscription{}
		err := scanSubscription(s, rows)
		if err != nil {
			log.Fatalf("failed to select subscriptions: %s", err)
		}
		subs = append(subs, s)
	}
	return subs
}

func (db *DB) UpdateSubscription(s *Subscription) {
	if s.ID <= 0 {
		log.Fatalf("invalid subscription ID %d", s.ID)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE subscription SET ip = $1, confirm = $2, target = $3 WHERE id = $4",
		s.IP,
		s.Confirm,
		s.Target,
		s.ID)
	if err != nil {
		log.Fatalf("failed to update subscription: %s", err)
	}
}

func (db *DB) DeleteSubscription(s *Subscription) {
	if s.ID == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM subscription WHERE id = $1", s.ID)
	if err != nil {
		log.Fatalf("failed to delete subscription: %s", err)
	}
}

func (db *DB) DeleteSubscriptionsByBoard(boardID int) {
	if boardID == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM subscription WHERE board = $1", boardID)
	if err != nil {
		log.Fatalf("failed to delete subscription: %s", err)
	}
}

func (db *DB) DeleteSubscriptionsByPost(postID int) {
	if postID == 0 {
		return
	}
	_, err := db.conn.Exec(context.Background(), "DELETE FROM subscription WHERE target = $1", postID)
	if err != nil {
		log.Fatalf("failed to delete subscription: %s", err)
	}
}

func scanSubscription(s *Subscription, row pgx.Row) error {
	return row.Scan(
		&s.ID,
		&s.IP,
		&s.Confirm,
		&s.Email,
		&s.Board,
		&s.Target,
	)
}
