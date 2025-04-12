package sriracha

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addAccount(a *Account, password string) error {
	_, err := db.conn.Exec(context.Background(), "INSERT INTO account VALUES (DEFAULT, $1, $2, $3, 0, '')", a.Username, db.hashData(password), a.Role)
	if err != nil {
		return fmt.Errorf("failed to insert account: %s", err)
	}
	return nil
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

func (db *Database) accountByID(id int) (*Account, error) {
	a := &Account{}
	var password string
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE id = $1", id).Scan(&a.ID, &a.Username, &password, &a.Role, &a.LastActive, &a.Session)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select account: %s", err)
	}
	return a, nil
}

func (db *Database) accountByUsername(username string) (*Account, error) {
	a := &Account{}
	var passwordHash string
	err := db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE username = $1 AND role != $2", username, RoleDisabled).Scan(&a.ID, &a.Username, &passwordHash, &a.Role, &a.LastActive, &a.Session)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to select account: %s", err)
	} else if a.ID == 0 {
		return nil, nil
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

func (db *Database) allAccounts() ([]*Account, error) {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM account ORDER BY role ASC, username ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to select accounts: %s", err)
	}
	var accounts []*Account
	for rows.Next() {
		a := &Account{}
		var password string
		err := rows.Scan(&a.ID, &a.Username, &password, &a.Role, &a.LastActive, &a.Session)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (db *Database) updateAccountUsername(a *Account) error {
	if a == nil || a.ID <= 0 {
		return fmt.Errorf("invalid account")
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET username = $1, session = '' WHERE id = $2", a.Username, a.ID)
	if err != nil {
		return fmt.Errorf("failed to update account: %s", err)
	}
	return nil
}

func (db *Database) updateAccountRole(a *Account) error {
	if a == nil || a.ID <= 0 {
		return fmt.Errorf("invalid account")
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET role = $1 WHERE id = $2", a.Role, a.ID)
	if err != nil {
		return fmt.Errorf("failed to update account: %s", err)
	}
	return nil
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

func (db *Database) updateAccountLastActive(id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid account ID %d", id)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET lastactive = $1 WHERE id = $2", time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("failed to update account: %s", err)
	}
	return nil
}

func (db *Database) loginAccount(username string, password string) (*Account, error) {
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
