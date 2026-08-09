//go:build nosqlite

package server

import "errors"

const builtWithSQLite = false

var errorMissingSQLite = errors.New("Error: Unable to import/export posts: Sriracha was compiled without support for reading and writing SQLite database files. Recompile Sriracha without the 'nosqlite' tag or use a supported platform to perform the import/export instead.")
