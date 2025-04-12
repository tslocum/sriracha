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

func connectDatabase(address string, username string, password string, schema string) (*pgxpool.Pool, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s/%s", username, password, address, schema)

	pool, err := pgxpool.New(context.Background(), url)
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

func (db *Database) createSuperAdminAccount() error {
	var numAdmins int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM account WHERE role = $1", RoleSuperAdmin).Scan(&numAdmins)
	if err != nil {
		return fmt.Errorf("failed to select number of super-administrator accounts: %s", err)
	} else if numAdmins > 0 {
		return nil
	}
	err = db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM account WHERE username = 'admin'").Scan(&numAdmins)
	if err != nil {
		return fmt.Errorf("failed to select number of super-administrator accounts: %s", err)
	} else if numAdmins > 0 {
		_, err = db.conn.Exec(context.Background(), "UPDATE account SET password = $1, role = $2, session = '' WHERE username = 'admin'", db.hashData("admin"), RoleSuperAdmin)
		if err != nil {
			return fmt.Errorf("failed to insert account: %s", err)
		}
		return nil
	}
	_, err = db.conn.Exec(context.Background(), "INSERT INTO account VALUES (DEFAULT, 'admin', $1, $2, 0, '')", db.hashData("admin"), RoleSuperAdmin)
	if err != nil {
		return fmt.Errorf("failed to insert account: %s", err)
	}
	return nil
}

func (db *Database) accountByUsernamePassword(username string, password string) (*Account, error) {
	a := &Account{}
	var passwordHash string
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE username = $1 AND role != $2", username, RoleDisabled).Scan(&a.ID, &a.Username, &passwordHash, &a.Role, &a.LastActive, &a.Session)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select account: %s", err)
	} else if a.ID == 0 || !db.compareHash(password, passwordHash) {
		return nil, nil
	}
	for {
		sessionKey := newSessionKey()
		var numAccounts int
		err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM account WHERE session = $1", sessionKey).Scan(&numAccounts)
		if err != nil {
			return nil, fmt.Errorf("failed to select number of accounts with session key: %s", err)
		} else if numAccounts == 0 {
			a.Session = sessionKey
			_, err = db.conn.Exec(context.Background(), "UPDATE account SET session = $1 WHERE id = $2", sessionKey, a.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to update account: %s", err)
			}
			break
		}
	}
	return a, nil
}

func (db *Database) accountBySessionKey(sessionKey string) (*Account, error) {
	if strings.TrimSpace(sessionKey) == "" {
		return nil, nil
	}

	a := &Account{}
	var passwordHash string
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE session = $1 AND role != $2", sessionKey, RoleDisabled).Scan(&a.ID, &a.Username, &passwordHash, &a.Role, &a.LastActive, &a.Session)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select account: %s", err)
	}
	return a, nil
}

func (db *Database) updateAccountPassword(id int, password string) error {
	if id <= 0 {
		return fmt.Errorf("invalid account ID %d", id)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET password = $1, session = '' WHERE id = $2", db.hashData(password), id)
	if err != nil {
		return fmt.Errorf("failed to update account: %s", err)
	}
	return nil
}

func (db *Database) addBoard(b *Board) error {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO board VALUES (DEFAULT, $1, $2, $3, $4)", b.Dir, b.Name, b.Description, b.Type)
	if err != nil {
		return fmt.Errorf("failed to insert board: %s", err)
	}
	return nil
}

func (db *Database) updateBoard(b *Board) error {
	if b.ID <= 0 {
		return fmt.Errorf("invalid board ID %d", b.ID)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE board SET dir = $1, name = $2, description = $3, type = $4 WHERE id = $5", b.Dir, b.Name, b.Description, b.Type, b.ID)
	if err != nil {
		return fmt.Errorf("failed to update board: %s", err)
	}
	return nil
}

func (db *Database) boardByID(id int) (*Board, error) {
	b := &Board{}
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM board ORDER BY dir ASC").Scan(&b.ID, &b.Dir, &b.Name, &b.Description, &b.Type)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select board: %s", err)
	}
	return b, nil
}

func (db *Database) allBoards() ([]*Board, error) {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM board ORDER BY dir ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to insert board: %s", err)
	}
	var boards []*Board
	for rows.Next() {
		b := &Board{}
		err := rows.Scan(&b.ID, &b.Dir, &b.Name, &b.Description, &b.Type)
		if err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, nil
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
