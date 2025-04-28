# Sriracha Board Migration
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

Sriracha supports migrating boards from [TinyIB](https://codeberg.org/tslocum/tinyib).

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

### IP address and file hashes are incompatible

Sriracha hashes IP addresses and files by generating a salted SHA384 checksum
of the data. TinyIB hashes IP addresses and files using crypt. Because of this,
bans are not imported, and posts will have their IP address field blanked. File
hashes are recalculated and corrected during import.

### All keywords are regular expressions

Sriracha keywords are always regular expressions. During migration, plain text
keywords are escaped to allow them to be parsed as regular expressions. You may
still need to update some keywords for them to continue to function.

### Previews only work for displayed posts

When hovering over a reference link, post previews are only shown for posts
already displayed on the page. Reference links to replies which not displayed
on the page, such as omitted replies when browsing board index pages, will not
show a preview. Open thread pages to show previews for all referenced replies.

### No backlinks

Backlinks are links to each post referencing another post, which are displayed
alongside post IDs. Sriracha does not support displaying backlinks.

### Licensed under GNU LGPL

Sriracha is licensed under [GNU LGPL](https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE).
If you modify the source code of this application, you must share the full
source code of your changes publicly for free. You may, however, link with this
application using proprietary shared libraries, so long as the base application
(Sriracha) remains unmodified. If your only changes are to create proprietary
shared libraries, and these librarires would work with other installations of
Sriracha because you did not make any modifications to Sriracha's source code,
then you do not need to release the source code of your shared libraries.

If you run an unmodified official release of this application, either by running
an official release binary or by compiling Sriracha using only the unmodified
source code of an official release, then you do not need to share any source code.

## Instructions

Posts and keywords will be imported into Sriracha. All other data is incompatible.

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
  address: localhost # Address:Port to connect to the database.
  username: tinyib   # Database username.
  password: hunter2  # Database password.
  dbname: tinyib     # Database name.
  # Table names.
  posts: dir_posts   # Required.
  keywords: keywords # Optional.
```

### 4. Start Sriracha and visit the management panel

Log in to the management panel as a super-administrator and follow the
on-screen prompts. After validating the import configuration, you may initiate
a dry run of the import. If the dry run is successful, you may then initiate
the actual import. You will be prompted for a board directory and name. The
`src` and `thumb` directories, containing all of the uploaded files, must exist
in the chosen board directory.

To migrate a single board, and continue running with only one board, copy `src`
and `thumb` to the root directory and leave the board directory field blank.

### 5. Restart Sriracha in normal mode

Remove the import configuration option from `config.yml` and restart Sriracha
to re-enable posting.

Don't forget to keep the backup handy, even if the migration appears to
be successful.
