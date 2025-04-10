package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type database struct {
	conn *pgx.Conn
}

func connectDatabase(address string, username string, password string, schema string) (*database, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s/%s", username, password, address, schema)

	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %s", err)
	}

	d := &database{
		conn: conn,
	}
	return d, nil
}
