package sriracha

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (db *Database) addAccount(a *Account, password string) {
	sessionKey := db.newSessionKey()
	_, err := db.conn.Exec(context.Background(), "INSERT INTO account VALUES (DEFAULT, $1, $2, $3, 0, $4)",
		a.Username,
		encryptPassword(password),
		a.Role,
		sessionKey,
	)
	if err != nil {
		log.Fatalf("failed to insert account: %s", err)
	}
	err = db.conn.QueryRow(context.Background(), "SELECT id FROM account WHERE username = $1", a.Username).Scan(&a.ID)
	if err != nil || a.ID == 0 {
		log.Fatalf("failed to select id of inserted account: %s", err)
	}
}

func (db *Database) createSuperAdminAccount() {
	var numAdmins int
	err := db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM account WHERE role = $1", RoleSuperAdmin).Scan(&numAdmins)
	if err != nil {
		log.Fatalf("failed to select number of super-administrator accounts: %s", err)
	} else if numAdmins > 0 {
		return
	}
	err = db.conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM account WHERE username = 'admin'").Scan(&numAdmins)
	if err != nil {
		log.Fatalf("failed to select number of super-administrator accounts: %s", err)
	} else if numAdmins > 0 {
		sessionKey := db.newSessionKey()
		_, err = db.conn.Exec(context.Background(), "UPDATE account SET password = $1, role = $2, session = $3 WHERE username = 'admin'",
			encryptPassword("admin"),
			RoleSuperAdmin,
			sessionKey,
		)
		if err != nil {
			log.Fatalf("failed to insert account: %s", err)
		}
		return
	}
	_, err = db.conn.Exec(context.Background(), "INSERT INTO account VALUES (DEFAULT, 'admin', $1, $2, 0, '')", encryptPassword("admin"), RoleSuperAdmin)
	if err != nil {
		log.Fatalf("failed to insert account: %s", err)
	}
}

func (db *Database) accountByID(id int) *Account {
	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE id = $1", id))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select account: %s", err)
	}
	return a
}

func (db *Database) accountByUsername(username string) *Account {
	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE username = $1 AND role != $2", username, RoleDisabled))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select account: %s", err)
	} else if a.ID == 0 {
		return nil
	}
	return a
}

func (db *Database) accountBySessionKey(sessionKey string) *Account {
	if strings.TrimSpace(sessionKey) == "" {
		return nil
	}

	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE session = $1 AND role != $2", sessionKey, RoleDisabled))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select account: %s", err)
	}
	return a
}

func (db *Database) allAccounts() []*Account {
	rows, err := db.conn.Query(context.Background(), "SELECT * FROM account ORDER BY role ASC, username ASC")
	if err != nil {
		log.Fatalf("failed to select accounts: %s", err)
	}
	var accounts []*Account
	for rows.Next() {
		a := &Account{}
		var password string
		err = rows.Scan(&a.ID, &a.Username, &password, &a.Role, &a.LastActive, &a.Session)
		if err != nil {
			log.Fatalf("failed to select accounts: %s", err)
		}
		accounts = append(accounts, a)
	}
	return accounts
}

func (db *Database) updateAccountUsername(a *Account) {
	if a == nil || a.ID <= 0 {
		log.Fatalf("invalid account")
	}
	sessionKey := db.newSessionKey()
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET username = $1, session = $2 WHERE id = $3", a.Username, sessionKey, a.ID)
	if err != nil {
		log.Fatalf("failed to update account: %s", err)
	}
}

func (db *Database) updateAccountRole(a *Account) {
	if a == nil || a.ID <= 0 {
		log.Fatalf("invalid account")
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET role = $1 WHERE id = $2", a.Role, a.ID)
	if err != nil {
		log.Fatalf("failed to update account: %s", err)
	}
}

func (db *Database) updateAccountPassword(id int, password string) {
	if id <= 0 {
		log.Fatalf("invalid account ID %d", id)
	}
	sessionKey := db.newSessionKey()
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET password = $1, session = $2 WHERE id = $3", encryptPassword(password), sessionKey, id)
	if err != nil {
		log.Fatalf("failed to update account: %s", err)
	}
}

func (db *Database) updateAccountLastActive(id int) {
	if id <= 0 {
		log.Fatalf("invalid account ID %d", id)
	}
	_, err := db.conn.Exec(context.Background(), "UPDATE account SET lastactive = $1 WHERE id = $2", time.Now().Unix(), id)
	if err != nil {
		log.Fatalf("failed to update account: %s", err)
	}
}

func (db *Database) loginAccount(username string, password string) *Account {
	a := &Account{}
	err := scanAccount(a, db.conn.QueryRow(context.Background(), "SELECT * FROM account WHERE username = $1 AND role != $2", username, RoleDisabled))
	if err == pgx.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatalf("failed to select account: %s", err)
	} else if a.ID == 0 || !comparePassword(password, a.Password) {
		return nil
	}
	a.Session = db.newSessionKey()
	_, err = db.conn.Exec(context.Background(), "UPDATE account SET session = $1 WHERE id = $2", a.Session, a.ID)
	if err != nil {
		log.Fatalf("failed to update account: %s", err)
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
	)
}
