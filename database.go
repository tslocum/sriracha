package sriracha

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var argon2idParameters = &argon2id.Params{
	Memory:      128 * 1024,
	Iterations:  2,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   64,
}

type Database struct {
	conn   *pgxpool.Conn
	plugin string
}

func connectDatabase(address string, username string, password string, schema string, poolMin int, poolMax int) (*pgxpool.Pool, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s/%s", username, password, address, schema)

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	config.MinConns = int32(poolMin)
	config.MinIdleConns = int32(poolMin)
	config.MaxConns = int32(poolMax)

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %s", err)
	}

	conn, err := pool.Acquire(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to acquire conn: %s", err)
	}
	defer conn.Release()

	db := &Database{
		conn: conn,
	}
	err = db.initialize(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %s", err)
	}
	err = db.upgrade()
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade database: %s", err)
	}
	err = db.createSuperAdminAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to create super-administrator account: %s", err)
	}
	return pool, nil
}

func (db *Database) initialize(schema string) error {
	_, err := db.conn.Exec(context.Background(), "SELECT 1=1")
	if err != nil {
		return fmt.Errorf("failed to test database connection: %s", err)
	}

	var tablecount int
	err = db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = $1 AND table_name = 'config'", schema).Scan(&tablecount)
	if err != nil {
		return fmt.Errorf("failed to select whether config table exists: %s", err)
	} else if tablecount > 0 {
		return nil
	}

	_, err = db.conn.Exec(context.Background(), dbSchema[0])
	if err != nil {
		return fmt.Errorf("failed to create database: %s", err)
	}
	return nil
}

func (db *Database) upgrade() error {
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
	for v := version + 1; v <= maxVersion; v++ {
		_, err = db.conn.Exec(context.Background(), dbSchema[v-1])
		if err != nil {
			return fmt.Errorf("failed to upgrade database to version %d: %s", v, err)
		}
	}
	return nil
}

func (db *Database) hashData(data string) string {
	hash, err := argon2id.CreateHash(data+srirachaServer.config.Salt, argon2idParameters)
	debug.FreeOSMemory() // Hashing is memory intensive. Return memory to the OS.
	if err != nil {
		log.Fatal(err)
	}
	return hash
}

func (db *Database) compareHash(data string, hash string) bool {
	match, err := argon2id.ComparePasswordAndHash(data+srirachaServer.config.Salt, hash)
	debug.FreeOSMemory() // Hashing is memory intensive. Return memory to the OS.
	if err != nil {
		log.Fatal(err)
	}
	return match
}

func (db *Database) configKey(key string) string {
	key = strings.ToLower(key)
	if len(db.plugin) != 0 {
		return db.plugin + "." + key
	}
	return key
}

func (db *Database) GetString(key string) (string, error) {
	key = db.configKey(key)
	var value string
	err := db.conn.QueryRow(context.Background(), "SELECT value FROM config WHERE name = $1", key).Scan(&value)
	if err == pgx.ErrNoRows {
		// TODO use default value
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("failed to get string %s: %s", key, err)
	}
	return value, nil
}

func (db *Database) GetMultiString(key string) ([]string, error) {
	value, err := db.GetString(key)
	if err != nil {
		return nil, err
	}
	return strings.Split(value, "|"), nil
}

func newSessionKey() string {
	const keyLength = 48
	buf := make([]byte, keyLength)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
