//go:build !nosqlite

package server

import (
	"errors"

	_ "modernc.org/sqlite"
)

const builtWithSQLite = true

var errorMissingSQLite = errors.New("")
