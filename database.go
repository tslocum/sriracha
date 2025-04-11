package sriracha

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
)

var argon2idParameters = &argon2id.Params{
	Memory:      128 * 1024,
	Iterations:  2,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   64,
}

type Database struct {
	conn   *pgx.Conn
	plugin string
}

func connectDatabase(address string, username string, password string, schema string) (*Database, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s/%s", username, password, address, schema)

	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %s", err)
	}

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
	return db, nil
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
		_, err = db.conn.Exec(context.Background(), "UPDATE account SET password = $1, role = $2", db.hashData("admin"), RoleSuperAdmin)
		if err != nil {
			return fmt.Errorf("failed to insert account: %s", err)
		}
		return nil
	}
	_, err = db.conn.Exec(context.Background(), "INSERT INTO account VALUES (DEFAULT, 'admin', $1, $2, $3)", db.hashData("admin"), RoleSuperAdmin, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to insert account: %s", err)
	}
	return nil
}

func (db *Database) accountByUsernamePassword(username string, password string) (*Account, error) {
	a := &Account{}
	var passwordHash string
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE username = $1 AND role != $2", username, RoleDisabled).Scan(&a.ID, &a.Username, &passwordHash, &a.Role, &a.LastActive)
	if err != nil {
		return nil, fmt.Errorf("failed to select account: %s", err)
	} else if a.ID == 0 || !db.compareHash(password, passwordHash) {
		return nil, nil
	}
	return a, nil
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
	if err != nil {
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
