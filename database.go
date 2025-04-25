package sriracha

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
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

func connectDatabase(address string, username string, password string, dbName string, poolMin int, poolMax int) (*pgxpool.Pool, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s/%s", username, password, address, dbName)

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

	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %s", err)
	}

	db := &Database{
		conn: conn,
	}
	err = db.initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %s", err)
	}

	err = db.upgrade()
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade database: %s", err)
	}

	db.createSuperAdminAccount()

	err = db.loadPluginConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin configuration values: %s", err)
	}

	_, err = conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %s", err)
	}
	return pool, nil
}

func (db *Database) initialize() error {
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

func (db *Database) loadPluginConfig() error {
	for _, info := range allPluginInfo {
		db.plugin = strings.ToLower(info.Name)
		for i, c := range info.Config {
			v := db.GetString(strings.ToLower(info.Name + "." + c.Name))
			if v != "" {
				info.Config[i].Value = v
			}
		}
	}
	db.plugin = ""
	return nil
}

func (db *Database) configKey(key string) string {
	key = strings.ToLower(key)
	if len(db.plugin) != 0 {
		return db.plugin + "." + key
	}
	return key
}

func (db *Database) HaveConfig(key string) bool {
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

func (db *Database) GetString(key string) string {
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

func (db *Database) GetMultiString(key string) []string {
	return strings.Split(db.GetString(key), "|")
}

func (db *Database) GetBool(key string) bool {
	return db.GetString(key) == "1"
}

func (db *Database) GetInt(key string) int {
	return parseInt(db.GetString(key))
}

func (db *Database) SaveString(key string, value string) {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO config VALUES ($1, $2) ON CONFLICT (name) DO UPDATE SET value = $3", db.configKey(key), value, value)
	if err != nil {
		log.Fatalf("failed to save string: %s", err)
	}
}

func (db *Database) SaveMultiString(key string, value []string) {
	db.SaveString(key, strings.Join(value, "|"))
}

func (db *Database) SaveBool(key string, value bool) {
	v := "0"
	if value {
		v = "1"
	}
	db.SaveString(key, v)
}

func (db *Database) SaveInt(key string, value int) {
	db.SaveString(key, strconv.Itoa(value))
}

func (db *Database) newSessionKey() string {
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
