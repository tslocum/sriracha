package database

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/alexedwards/argon2id"
	"github.com/gabriel-vasile/mimetype"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var argon2idParameters = &argon2id.Params{
	Memory:      128 * 1024,
	Iterations:  2,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   64,
}

// DB represents a database connection.
type DB struct {
	conn   *pgxpool.Conn
	Plugin string
	config *Config
}

func Connect(c *Config) (*pgxpool.Pool, error) {
	url := c.DBURL
	if strings.TrimSpace(url) == "" {
		url = fmt.Sprintf("postgres://%s:%s@%s/%s", c.Username, c.Password, c.Address, c.DBName)
	}

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database configuration: %s", err)
	}
	config.MinConns = 1
	config.MinIdleConns = 1
	config.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %s", err)
	}

	conn, err := pool.Acquire(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to acquire conn: %s", err)
	}
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %s", err)
	}

	db := &DB{
		conn:   conn,
		config: c,
	}
	err = db.initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %s", err)
	}

	err = db.upgrade(c.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade database: %s", err)
	}

	db.createSuperAdminAccount(c.SaltPass)

	_, err = conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %s", err)
	}
	return pool, nil
}

func (db *DB) initialize() error {
	_, err := db.conn.Exec(context.Background(), "SELECT 1=1")
	if err != nil {
		return fmt.Errorf("failed to test database connection: %s", err)
	}

	var tablecount int
	err = db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'account'").Scan(&tablecount)
	if err != nil {
		return fmt.Errorf("failed to select whether account table exists: %s", err)
	} else if tablecount > 0 {
		return nil
	}

	fmt.Printf("Initializing database version 1...\n")
	_, err = db.conn.Exec(context.Background(), dbSchema[0])
	if err != nil {
		return fmt.Errorf("failed to create database: %s", err)
	}
	fmt.Printf("Database initialized.\n")
	return nil
}

func (db *DB) _upgrade(rootDir string, v int) error {
	_, err := db.conn.Exec(context.Background(), dbSchema[v-1])
	if err != nil {
		return err
	}
	switch v {
	case 5: // Add file MIME type to posts.
		boards := db.AllBoards()
		for _, b := range boards {
			allThreads := db.AllThreads(b, false)
			for _, threadInfo := range allThreads {
				posts := db.AllPostsInThread(threadInfo[0], false)
				for _, post := range posts {
					if post.File != "" && !post.IsEmbed() {
						if strings.HasSuffix(post.File, ".tgkr") {
							post.FileMIME = "application/x-tegaki"
						} else {
							mimeInfo, err := mimetype.DetectFile(filepath.Join(rootDir, b.Dir, "src", post.File))
							if err == nil {
								post.FileMIME = mimeInfo.String()
							}
						}
						if post.FileMIME != "" {
							_, err = db.conn.Exec(context.Background(), "UPDATE post SET filemime = $1 WHERE id = $2", post.FileMIME, post.ID)
							if err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func (db *DB) upgrade(rootDir string) error {
	var versionString string
	err := db.conn.QueryRow(context.Background(), "SELECT value FROM config WHERE name = 'version'").Scan(&versionString)
	if err != nil {
		return fmt.Errorf("failed to select database version: %s", err)
	}
	version, err := strconv.Atoi(versionString)
	if err != nil {
		return fmt.Errorf("failed to parse database version: %s", err)
	}
	maxVersion := len(dbSchema)
	if version == maxVersion {
		return nil
	} else if version > maxVersion {
		return fmt.Errorf("database version %d is newer than application version %d", version, maxVersion)
	}
	fmt.Printf("Upgrading database from version %d to %d...\n", version, maxVersion)
	for v := version + 1; v <= maxVersion; v++ {
		err = db._upgrade(rootDir, v)
		if err != nil {
			return fmt.Errorf("failed to upgrade database from version %d to version %d: %s", v-1, v, err)
		}
	}
	fmt.Printf("Database upgraded.\n")
	return nil
}

func Begin(pool *pgxpool.Pool, config *Config) *DB {
	if pool == nil {
		// Return mock database.
		return &DB{
			config: config,
		}
	}

	conn, err := pool.Acquire(context.Background())
	if err != nil {
		log.Fatalf("failed to acquire connection from pool: %s", err)
	}

	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		conn.Release()
		log.Fatalf("failed to begin transaction: %s", err)
	}

	return &DB{
		conn:   conn,
		config: config,
	}
}

func (db *DB) RollBack() {
	if db.conn == nil {
		return
	}
	_, err := db.conn.Exec(context.Background(), "ROLLBACK")
	if err != nil {
		log.Fatalf("failed to rollback transaction: %s", err)
	}
	db.conn.Release()
}

func (db *DB) Commit() {
	if db.conn == nil {
		return
	}
	_, err := db.conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		log.Fatalf("failed to commit transaction: %s", err)
	}
	db.conn.Release()
}

func (db *DB) CommitWithErr() error {
	if db.conn == nil {
		return nil
	}
	_, err := db.conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %s", err)
	}
	db.conn.Release()
	return nil
}

func (db *DB) configKey(key string) string {
	key = strings.ToLower(key)
	if len(db.Plugin) != 0 {
		return db.Plugin + "." + key
	}
	return key
}

func (db *DB) HaveConfig(key string) bool {
	if db.conn == nil {
		return false
	}
	key = db.configKey(key)
	var count int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM config WHERE name = $1", key).Scan(&count)
	if err == pgx.ErrNoRows {
		return false
	} else if err != nil {
		log.Fatalf("failed to select config count %s: %s", key, err)
	}
	return count > 0
}

func (db *DB) GetString(key string) string {
	if db.conn == nil {
		return ""
	}
	key = db.configKey(key)
	var value string
	err := db.conn.QueryRow(context.Background(), "SELECT value FROM config WHERE name = $1", key).Scan(&value)
	if err == pgx.ErrNoRows {
		return ""
	} else if err != nil {
		log.Fatalf("failed to get string %s: %s", key, err)
	}
	return value
}

func (db *DB) SaveString(key string, value string) {
	if db.conn == nil {
		return
	}
	value = strings.ReplaceAll(value, "\r", "")
	_, err := db.conn.Exec(context.Background(), "INSERT INTO config VALUES ($1, $2) ON CONFLICT (name) DO UPDATE SET value = $3", db.configKey(key), value, value)
	if err != nil {
		log.Fatalf("failed to save string: %s", err)
	}
}

func (db *DB) GetMultiString(key string) []string {
	return strings.Split(db.GetString(key), "|||")
}

func (db *DB) GetBool(key string) bool {
	return db.GetString(key) == "1"
}

func (db *DB) SaveBool(key string, value bool) {
	v := "0"
	if value {
		v = "1"
	}
	db.SaveString(key, v)
}

func (db *DB) SaveMultiString(key string, value []string) {
	db.SaveString(key, strings.Join(value, "|||"))
}

func (db *DB) GetInt(key string) int {
	return ParseInt(db.GetString(key))
}

func (db *DB) GetInt64(key string) int64 {
	return ParseInt64(db.GetString(key))
}

func (db *DB) GetMultiInt(key string) []int {
	s := db.GetString(key)
	if s == "" {
		return nil
	}
	var values []int
	for _, v := range strings.Split(s, "|||") {
		values = append(values, ParseInt(v))
	}
	return values
}

func (db *DB) SaveInt(key string, value int) {
	db.SaveString(key, strconv.Itoa(value))
}

func (db *DB) GetFloat(key string) float64 {
	return ParseFloat(db.GetString(key))
}

func (db *DB) SaveFloat(key string, value float64) {
	db.SaveString(key, fmt.Sprintf("%f", value))
}

func (db *DB) newSessionKey() string {
	const keyLength = 48
	buf := make([]byte, keyLength)
	for {
		_, err := rand.Read(buf)
		if err != nil {
			panic(err)
		}
		sessionKey := base64.URLEncoding.EncodeToString(buf)

		var numAccounts int
		err = db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM account WHERE session = $1", sessionKey).Scan(&numAccounts)
		if err != nil {
			log.Fatalf("failed to select number of accounts with session key: %s", err)
		} else if numAccounts == 0 {
			return sessionKey
		}
	}
}

func (db *DB) Exec(sql string, arguments ...any) (pgconn.CommandTag, error) {
	return db.conn.Exec(context.Background(), sql, arguments...)
}

func (db *DB) QueryRow(sql string, arguments ...any) pgx.Row {
	return db.conn.QueryRow(context.Background(), sql, arguments...)
}

// Validate database interface during compilation.
var (
	_ sriracha.DB = &DB{}
)
