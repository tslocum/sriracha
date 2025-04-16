package sriracha

import (
	"context"
	"fmt"
)

func (db *Database) addPost(b *Board, p *Post) error {
	err := db.conn.QueryRow(context.Background(), "INSERT INTO post VALUES (DEFAULT, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24) RETURNING id",
		p.Parent,
		b.ID,
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
		return fmt.Errorf("failed to insert post: %s", err)
	}
	return nil
}
