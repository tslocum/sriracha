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
	err := db.conn.QueryRow(context.Background(), "INSERT INTO account VALUES (DEFAULT, $1, $2, $3, 0, $4, $5) RETURNING id",
		a.Username,
		encryptPassword(db.config.SaltPass, password),
		a.Role,
		a.Style,
		a.Locale,
	).Scan(&a.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert account: %w", err))
	}
}

func (db *DB) deleteAccountSessions(accountID int) {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM account_session WHERE account = $1", accountID)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete account sessions: %w", err))
	}
}

func (db *DB) createSuperAdminAccount() {
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
		a := db.AccountByUsername("admin")
		db.UpdateAccountPassword(a, "admin")
		a.Role = RoleSuperAdmin
		db.UpdateAccountRole(a)
		return
	}
	a := &Account{
		Username: "admin",
		Role:     RoleSuperAdmin,
	}
	db.AddAccount(a, "admin")
}

func (db *DB) addAccountSession(accountID int) string {
	sessionKey := db.newSessionKey()
	_, err := db.conn.Exec(context.Background(), "INSERT INTO account_session VALUES ($1, $2, $3)",
		sessionKey,
		accountID,
		time.Now().Unix(),
	)
	if err != nil {
		dbErr(fmt.Errorf("failed to insert account session: %w", err))
	}
	_, err = db.conn.Exec(context.Background(), "DELETE FROM account_session WHERE key IN (SELECT key FROM account_session WHERE account = $1 ORDER BY lastactive DESC OFFSET $2)", accountID, db.config.SessionLimit)
	if err != nil {
		dbErr(fmt.Errorf("failed to enforce account session limit: %w", err))
	}
	return sessionKey
}
func (db *DB) AccountSessionKeys(accountID int) []string {
	rows, err := db.conn.Query(context.Background(), "SELECT key FROM account_session")
	if err != nil {
		dbErr(fmt.Errorf("failed to select account sessions: %w", err))
	}
	var keys []string
	for rows.Next() {
		var key string
		err := rows.Scan(&key)
		if err != nil {
			dbErr(fmt.Errorf("failed to select account sessions: %w", err))
		}
		keys = append(keys, key)
	}
	if rows.Err() != nil {
		dbErr(fmt.Errorf("failed to select account sessions: %w", rows.Err()))
	}
	return keys
}

func (db *DB) ExpireAccountSessions() {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM account_session WHERE lastactive <= $1", time.Now().Unix()-db.config.SessionTime)
	if err != nil {
		dbErr(fmt.Errorf("failed to expire account sessions: %w", err))
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

	var accountID int
	err := db.conn.QueryRow(context.Background(), "SELECT account FROM account_session WHERE key = $1", sessionKey).Scan(&accountID)
	if err == pgx.ErrNoRows || accountID <= 0 {
		return nil
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select account by session: %w", err))
	}
	a := db.AccountByID(accountID)
	if a == nil || a.Role == RoleDisabled {
		return nil
	}
	db.UpdateAccountLastActive(a.ID)
	_, err = db.conn.Exec(context.Background(), "UPDATE account_session SET lastactive = $1 WHERE key = $2", time.Now().Unix(), sessionKey)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account session last active timestamp: %w", err))
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
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET username = $1 WHERE id = $2", a.Username, a.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
	db.deleteAccountSessions(a.ID)
}

func (db *DB) UpdateAccountRole(a *Account) {
	if a == nil || a.ID <= 0 {
		dbErr(fmt.Errorf("invalid account: %v", a))
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET role = $1 WHERE id = $2", a.Role, a.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
	db.deleteAccountSessions(a.ID)
}

func (db *DB) UpdateAccountPassword(a *Account, password string) {
	if a.ID <= 0 {
		dbErr(fmt.Errorf("invalid account ID %d", a.ID))
	}
	a.Password = encryptPassword(db.config.SaltPass, password)
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET password = $1 WHERE id = $2", a.Password, a.ID)
	if err != nil {
		dbErr(fmt.Errorf("failed to update account: %w", err))
	}
	db.deleteAccountSessions(a.ID)
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

func (db *DB) LoginAccount(username string, password string, newSession bool) (*Account, string) {
	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE username = $1 AND role != $2", username, RoleDisabled))
	if err == pgx.ErrNoRows {
		return nil, ""
	} else if err != nil {
		dbErr(fmt.Errorf("failed to select account: %w", err))
	} else if a.ID == 0 || !comparePassword(db.config.SaltPass, password, a.Password) {
		return nil, ""
	}
	var key string
	if newSession {
		key = db.addAccountSession(a.ID)
	}
	return a, key
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

func (db *DB) DeleteAccountSession(key string) {
	_, err := db.conn.Exec(context.Background(), "DELETE FROM account_session WHERE key = $1", key)
	if err != nil {
		dbErr(fmt.Errorf("failed to delete account session: %w", err))
	}
}

func scanAccount(a *Account, row pgx.Row) error {
	return row.Scan(
		&a.ID,
		&a.Username,
		&a.Password,
		&a.Role,
		&a.LastActive,
		&a.Style,
		&a.Locale,
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
