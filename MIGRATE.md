# Sriracha Board Migration
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

This application is in pre-alpha development. Don't migrate any boards yet.

Sriracha supports migrating one or more boards from [TinyIB](https://codeberg.org/tslocum/tinyib).

## Differences

### Only PostgreSQL is supported

Sriracha only supports the [PostgreSQL](https://www.postgresql.org) database system.

### Account roles have different capabilities

See [README.md](https://codeberg.org/tslocum/sriracha/src/branch/main/README.md)
for more info on the capabilities of each role.

### Single auto-increment post ID

Sriracha uses one auto-incrementing post ID for all boards. Because of this,
migrating two more more boards will involve changing each post's ID. Reference
links inside posts are updated, but external links to old res pages will break.

### Incompatible IP address and file hashes

Sriracha hashes IP addresses and files by generating a salted SHA384 checksum
of the data. TinyIB hashes IP addresses and files using crypt. Because of this,
bans are not imported, and posts will have their IP address field blanked. File
hashes are recalculated and corrected during import.

### All keywords are regular expressions

Sriracha keywords are always regular expressions. During migration, plain text
keywords are escaped to allow them to be parsed as regular expressions. You may
still need to update some keywords for them to continue to function.

## Instructions

**Note:** Don't do this yet. These instructions won't work.

Posts, keywords and logs will be imported into Sriracha.

### 1. Back everything up

Before going any further, back everything up on the server. This includes files
and databases, if an external database like MySQL or PostgreSQL was used.

Store the backup somewhere other than the server, such as your computer's hard
drive. Keep this backup handy, even if the migration appears to be successful.

### 2. Migrate TinyIB to PostgreSQL

If you are already using PostgreSQL as your TinyIB database, you may skip this step.

Use TinyIB's built in database migration tool to migrate your database to PostgreSQL.
This may require migrating to an intermediate database, depending on which `TINYIB_DBMODE`
is in use.

#### A. Migrate to SQLite or flat file

If your `TINYIB_DBMODE` is set to `sqlite`, `sqlite3` or `flatfile`, skip to part B.

Set `TINYIB_DBMIGRATE` to `sqlite3` and follow the [migration instructions](https://codeberg.org/tslocum/tinyib#migrate).
If `sqlite3` does not work, try `sqlite`. As a last resort, `flatfile` may be used
as the intermediate database. Set any relevant database configuration options
before migrating.

Once you have migrated your database to `sqlite`, `sqlite3` or `flatfile`, proceed to part B.

#### B. Migrate to PDO

Set `TINYIB_DBMIGRATE` to `pdo`, `TINYIB_DBDRIVER` to `pgsql` and follow the [migration instructions](https://codeberg.org/tslocum/tinyib#migrate).
Set any relevant database configuration options to the new PostgreSQL database.
This database will be read and imported by Sriracha.

### 3. Configure Sriracha to run in import mode

Add the following to your Sriracha `config.yml`, replacing the example values
with your TinyIB PostgreSQL database connection info and table names.

```yaml
# Note: Posting is disabled when running in import mode.
import:
  # Connection info.
  address: localhost
  username: tinyib
  password: hunter2
  dbname: tinyib
  # Table names.
  posts: dir_posts
  keywords: keywords
  logs: logs
```

### 4. Start Sriracha and visit the management panel

Log in to the management panel as a super-administrator and follow the
on-screen prompts to complete the board migration.

### 5. Restart Sriracha in normal mode

Remove the import configuration option from `config.yml` and then restart
Sriracha to re-enable posting.

Don't forget to keep the backup handy, even if the migration appears to
be successful.
