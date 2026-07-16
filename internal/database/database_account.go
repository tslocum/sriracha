package database

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
)

func (db *DB) AddAccount(a *Account, password string) {
	sessionKey := db.newSessionKey()
	err := db.conn.QueryRow(context.Background(), "INSERT INTO account VALUES (DEFAULT, $1, $2, $3, 0, $4, $5, $6, DEFAULT) RETURNING id",
		a.Username,
		encryptPassword(db.config.SaltPass, password),
		a.Role,
		sessionKey,
		a.Style,
		a.Locale,
	).Scan(&a.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert account: %w", err))
	}
}

func (db *DB) createSuperAdminAccount(salt string) {
	var numAdmins int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM account WHERE role = $1", RoleSuperAdmin).Scan(&numAdmins)
	if err != nil {
		dbErr(fmt.Errorf("failed to select number of super-administrator accounts: %w", err))
	} else if numAdmins > 0 {
		return
	}
	err = db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM account WHERE username = 'admin'").Scan(&numAdmins)
	if err != nil {
		dbErr(fmt.Errorf("failed to select number of super-administrator accounts: %w", err))
	} else if numAdmins > 0 {
		sessionKey := db.newSessionKey()
		_, err = db.conn.Exec(context.Background(), "UPDATE account SET password = $1, role = $2, session = $3 WHERE username = 'admin'",
			encryptPassword(salt, "admin"),
			RoleSuperAdmin,
			sessionKey,
		)
		if err != nil {
			dbErr(fmt.Errorf("failed to insert account: %w", err))
		}
		return
	}
	_, err = db.conn.Exec(context.Background(), "INSERT INTO account VALUES (DEFAULT, 'admin', $1, $2, 0, '')", encryptPassword(db.config.SaltPass, "admin"), RoleSuperAdmin)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert account: %w", err))
	}
}

func (db *DB) AccountByID(id int) *Account {
	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select account: %w", err))
	}
	return a
}

func (db *DB) AccountByUsername(username string) *Account {
	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE username = $1 AND role != $2", username, RoleDisabled))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		dbErr(fmt.Errorf("failed to select account: %w", err))
	}
	return a
}

func (db *DB) AccountBySessionKey(sessionKey string) *Account {
	if strings.TrimSpace(sessionKey) == "" {
		return nil
	}

	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE session = $1 AND role != $2", sessionKey, RoleDisabled))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select account: %w", err))
	}
	return a
}

func (db *DB) AllAccounts() []*Account {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM account ORDER BY role ASC, username ASC")
	if err != nil {
		dbErr(fmt.Errorf("failed to select accounts: %w", err))
	}
	var accounts []*Account
	for rows.Next() {
		a := &Account{}
		err = scanAccount(a, rows)
		if err != nil {
			dbErr(fmt.Errorf("failed to select accounts: %w", err))
		}
		accounts = append(accounts, a)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select accounts: %w", rows.Err()))
	}
	return accounts
}

func (db *DB) UpdateAccountUsername(a *Account) {
	if a == nil || a.ID <= 0 {
		dbErr(fmt.Errorf("invalid account: %v", a))
	}
	sessionKey := db.newSessionKey()
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET username = $1, session = $2 WHERE id = $3", a.Username, sessionKey, a.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
}

func (db *DB) UpdateAccountRole(a *Account) {
	if a == nil || a.ID <= 0 {
		dbErr(fmt.Errorf("invalid account: %v", a))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET role = $1 WHERE id = $2", a.Role, a.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
}

func (db *DB) UpdateAccountPassword(id int, password string) {
	if id <= 0 {
		dbErr(fmt.Errorf("invalid account ID %d", id))
	}
	sessionKey := db.newSessionKey()
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET password = $1, session = $2 WHERE id = $3", encryptPassword(db.config.SaltPass, password), sessionKey, id)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
}

func (db *DB) UpdateAccountLastActive(id int) {
	if id <= 0 {
		dbErr(fmt.Errorf("invalid account ID %d", id))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET lastactive = $1 WHERE id = $2", time.Now().Unix(), id)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
}

func (db *DB) UpdateAccountStyle(id int, style string) {
	if id <= 0 {
		dbErr(fmt.Errorf("invalid account ID %d", id))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET style = $1 WHERE id = $2", style, id)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
}

func (db *DB) UpdateAccountLocale(id int, locale string) {
	if id <= 0 {
		dbErr(fmt.Errorf("invalid account ID %d", id))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET locale = $1 WHERE id = $2", locale, id)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
}

func (db *DB) LoginAccount(username string, password string) *Account {
	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE username = $1 AND role != $2", username, RoleDisabled))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select account: %w", err))
	} else if a.ID == 0 || !comparePassword(db.config.SaltPass, password, a.Password) {
		return nil
	}
	a.Session = db.newSessionKey()
	_, err = db.conn.Exec(context.Background(), "UPDATE account SET session = $1 WHERE id = $2", a.Session, a.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
	return a
}

func (db *DB) CheckAccountPassword(username string, password string) *Account {
	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE username = $1 AND role != $2", username, RoleDisabled))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select account: %w", err))
	} else if a.ID == 0 || !comparePassword(db.config.SaltPass, password, a.Password) {
		return nil
	}
	return a
}

func scanAccount(a *Account, row pgx.Row) error {
	return row.Scan(
		&a.ID,
		&a.Username,
		&a.Password,
		&a.Role,
		&a.LastActive,
		&a.Session,
		&a.Style,
		&a.Locale,
		&a.ScratchCodes,
	)
}

func encryptPassword(salt string, password string) string {
	hash, err := argon2id.CreateHash(password+salt, argon2idParameters)
	debug.FreeOSMemory() // Hashing is memory intensive. Return memory to the OS.
	if err != nil {
		dbErr(err)
	}
	return hash
}

func comparePassword(salt string, password string, hash string) bool {
	match, err := argon2id.ComparePasswordAndHash(password+salt, hash)
	debug.FreeOSMemory() // Hashing is memory intensive. Return memory to the OS.
	if err != nil {
		dbErr(err)
	}
	return match
}
