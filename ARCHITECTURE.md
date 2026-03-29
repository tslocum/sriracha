# Sriracha Architecture
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

## Layout

The source code of Sriracha is organized as follows:

| Directory | Synopsis |
| --        | --       |
| [sriracha](https://codeberg.org/tslocum/sriracha) (root) | Imported by plugins to interact with the database. |
| [internal/database](https://codeberg.org/tslocum/sriracha/src/branch/main/internal/database) | Provides methods for interacting with the database. |
| [internal/server](https://codeberg.org/tslocum/sriracha/src/branch/main/internal/server) | Sriracha web server. |
| [internal/server/locale](https://codeberg.org/tslocum/sriracha/src/branch/main/internal/server/locale) | [Gettext](https://en.wikipedia.org/wiki/Gettext) locale files. |
| [internal/server/template](https://codeberg.org/tslocum/sriracha/src/branch/main/internal/server/template) | [Go HTML](https://pkg.go.dev/html/template) template files.
| [model](https://codeberg.org/tslocum/sriracha/src/branch/main/model) | Sriracha data types. |
| [util](https://codeberg.org/tslocum/sriracha/src/branch/main/util) | Sriracha utility functions and variables. |

## Design

Sriracha maintains a [PostgreSQL](https://www.postgresql.org) connection pool containing one connection.

When initializing a new Sriracha database, the version 1 schema is applied to the database, then upgraded to version 2, and so on.

When upgrading an existing Sriracha database, schema changes are applied automatically.

When a web request is received, a database connection is obtained from the connection pool before processing the request.

This connection will typically be held until the server finishes processing the request.

Some requests will release the database connection early, but only when it is safe to do so.

This allows Sriracha to safely handle multiple web requests simultaneously.

Whenever Sriracha data is modified, static HTML files are written to the configured root directory.

When a static file server is used in conjunction with Sriracha, visitors will only connect to
the static file server unless they create or delete a post.

Because of this, it is possible to make use of extensive server-side and client-side caching.
