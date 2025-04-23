package sriracha

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addCAPTCHA(c *CAPTCHA) {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO captcha VALUES ($1, $2, $3, $4, $5)",
		c.IP,
		c.Timestamp,
		c.Refresh,
		c.Image,
		c.Text,
	)
	if err != nil {
		log.Fatalf("failed to insert captcha: %s", err)
	}
}

func (db *Database) getCAPTCHA(ip string) *CAPTCHA {
	c := &CAPTCHA{}
	err := scanCAPTCHA(c, db.conn.QueryRow(context.Background(), "SELECT * FROM captcha WHERE ip = $1", ip))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select captcha: %s", err)
	}
	return c
}

func (db *Database) updateCAPTCHA(c *CAPTCHA) {
	_, err := db.conn.Exec(context.Background(), "UPDATE captcha SET refresh = $1, image = $2, text = $3 WHERE ip = $4", c.Refresh, c.Image, c.Text, c.IP)
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Database) expiredCAPTCHAs() []*CAPTCHA {
	const oneDay = 60 * 60 * 24
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM captcha WHERE timestamp <= $1", time.Now().Unix()-oneDay)
	if err != nil {
		log.Fatalf("failed to select expired captchas: %s", err)
	}
	var captchas []*CAPTCHA
	for rows.Next() {
		c := &CAPTCHA{}
		err := scanCAPTCHA(c, rows)
		if err != nil {
			log.Fatalf("failed to select expired captchas: %s", err)
		}
		captchas = append(captchas, c)
	}
	return captchas
}

func (db *Database) deleteCAPTCHA(ip string) {
	if ip == "" {
		return
	}

	_, err := db.conn.Exec(context.Background(), "DELETE FROM captcha WHERE ip = $1", ip)
	if err != nil {
		log.Fatalf("failed to delete captcha: %s", err)
	}
}

func (db *Database) newCAPTCHAImage() string {
	const keyLength = 48
	buf := make([]byte, keyLength)
	for {
		_, err := rand.Read(buf)
		if err != nil {
			panic(err)
		}
		imageName := base64.URLEncoding.EncodeToString(buf)

		var count int
		err = db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM captcha WHERE image = $1", imageName).Scan(&count)
		if err != nil {
			log.Fatalf("failed to select number of accounts with session key: %s", err)
		} else if count == 0 {
			return imageName
		}
	}
}

func scanCAPTCHA(c *CAPTCHA, row pgx.Row) error {
	return row.Scan(
		&c.IP,
		&c.Timestamp,
		&c.Refresh,
		&c.Image,
		&c.Text,
	)
}
