# Sriracha Manual
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

### Sections

- [**Install**](#install)
- [**Configure**](#configure)
- [**Migrate**](#migrate)
- [**Upgrade**](#upgrade)
- [**Plugins**](#plugins)
- [**Guides**](#guides)

## Install

[Go to top](#sections)

Only Linux, FreeBSD and macOS are supported.

### 1. Create root directory

Create a directory where board files will be written to. A new sub directory is
created for each board, except when a board is created with a blank directory.
Blank directory boards are useful when your site will only have one board.

### 2. Install PostgreSQL

Install the [PostgreSQL](https://www.postgresql.org/) database system by
following the relevant [documentation](https://www.postgresql.org/docs/current/admin.html).

### 3. Configure PostgreSQL

Create a new PostgreSQL database and role. Grant the new role access to the database
and set a password.

### 4. Download Sriracha

Download the [latest release](https://codeberg.org/tslocum/sriracha/releases) of
Sriracha for your platform.

Linux release archives include all official plugins. To use plugins on FreeBSD
or macOS, compile Sriracha and any desired plugins using the release source code.

## Configure

[Go to top](#sections)

When starting Sriracha, the path to the server configuration file may be
specified via the `--config` option:

`sriracha --config /path/to/config.yml`

If no configuration file path is specified, the default path
`~/.config/sriracha/config.yml` is used.

The timezone may be specified via the `TZ` environment variable:

`TZ=America/Los_Angeles sriracha`

[PostgreSQL](https://www.postgresql.org) is the only supported database system.

Sriracha serves requests at `/`, the root path. It is not currently possible to
run Sriracha under a subdirectory. Use a domain or subdomain to separate
Sriracha from other resources.

Only HTTP requests are served by Sriracha. To serve HTTPS requests you must run
Sriracha behind a web server, such as [caddy](https://caddyserver.com), which
forwards the HTTPS requests to Sriracha as plain HTTP. When running behind a web
server, the header server option must be set appropriately. Most web servers use
`X-Forwarded-For` to specify the client IP address.

Only requests to `/sriracha/*` need to be served by Sriracha. After copying
`static` to the root directory, you may handle all requests except `/sriracha/*`
using a static file server.

When starting Sriracha for the first time, visit the management panel at
`/sriracha/` and log in to the default super-administrator account by entering
`admin` as the username and the password. Once you have logged in, visit the
accounts page and change your username and password.

### Example configuration (config.yml)

```yaml
# Interface language. See locale directory for available languages.
locale: "en"

# Directory where board files are written to.
root: "/home/sriracha/public_html"

# Address:Port to listen for HTTP connections on.
serve: "localhost:8080"

# Client IP address header. Must be set when running behind a reverse proxy.
# When running behind CloudFlare, use CF-Connecting-IP. When running without
# a proxy, leave blank.
header: "X-Forwarded-For"

# Long random string of text used when one-way hashing data. Must not change once set.
saltdata: "CHANGEME_Random_Data_Here_1"

# Long random string of text used when two-way hashing data. Must not change once set.
saltpass: "CHANGEME_Random_Data_Here_2"

# Long random string of text used when generating secure tripcodes. Must not change once set.
salttrip: "CHANGEME_Random_Data_Here_3"

# Address:Port to connect to the database.
address: "localhost"

# Database username.
username: "sriracha"

# Database password.
password: "hunter2"

# Database name.
dbname: "sriracha"

# Database connection URL. Allows specifying additional connection options.
# This option supercedes the address, username, password and dbname options.
# See https://pkg.go.dev/github.com/jackc/pgx/v5@v5.7.4/pgconn#ParseConfig
#dburl: "postgresql://sriracha:hunter2@localhost/sriracha"

# Custom template directory. Leave blank to use standard templates. Template
# files in this directory will override standard templates of the same name.
template: "/home/sriracha/template"

# Supported upload file types. Specify a file extension and a MIME type to
# enable uploading files of that type. You may specify an image to use as the
# thumbnail for all uploads of that type, or 'none' to not create a thumbnail.
# Otherwise, thumbnails are generated automatically based on the uploaded file.
# To generate thumbnails for videos or SVG images, ffmpeg must be installed.
#
# Format: "ext mime thumbnail
uploads:
  - "jpg image/jpeg"
  - "jpg image/pjpeg"
  - "png image/png"
  - "gif image/gif"
  - "svg image/svg+xml"
  - "wav audio/wav"
  - "wav audio/wave"
  - "wav audio/x-wav"
  - "aac audio/aac"
  - "ogg audio/ogg"
  - "flac audio/flac"
  - "opus audio/opus"
  - "mp3 audio/mp3"
  - "mp3 audio/mpeg"
  - "mp4 audio/mp4"
  - "mp4 video/mp4"
  - "webm audio/webm"
  - "webm video/webm"
  - "swf application/x-shockwave-flash swf.png"
```

### Example reverse proxy using caddy (Caddyfile)

```
zoopz.org, www.zoopz.org {
  reverse_proxy http://localhost:8080
}
```

## Migrate

[Go to top](#sections)

Sriracha supports migrating boards from [TinyIB](https://codeberg.org/tslocum/tinyib).

### Differences

#### Only PostgreSQL is supported

Sriracha only supports the [PostgreSQL](https://www.postgresql.org) database system.

#### Account roles have different capabilities

See [README.md](https://codeberg.org/tslocum/sriracha/src/branch/main/README.md)
for more info on the capabilities of each role.

#### Single auto-increment post ID

Sriracha uses one auto-incrementing post ID for all boards. Because of this,
migrating two more more boards will involve changing each post's ID. Reference
links inside posts are updated, but external links to old res pages will break.

#### IP address and file hashes are incompatible

Sriracha hashes IP addresses and files by generating a salted SHA384 checksum
of the data. TinyIB hashes IP addresses and files using crypt. Because of this,
bans are not imported, and posts will have their IP address field blanked. File
hashes are recalculated and corrected during import.

#### All keywords are regular expressions

Sriracha keywords are always regular expressions. During migration, plain text
keywords are escaped to allow them to be parsed as regular expressions. You may
still need to update some keywords for them to continue to function.

#### Previews only work for displayed posts

When hovering over a reference link, post previews are only shown for posts
already displayed on the page. Reference links to replies which not displayed
on the page, such as omitted replies when browsing board index pages, will not
show a preview. Open thread pages to show previews for all referenced replies.

#### No backlinks

Backlinks are links to each post referencing another post, which are displayed
alongside post IDs. Sriracha does not support displaying backlinks.

#### Licensed under GNU LGPL

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

### Instructions

Posts and keywords will be imported into Sriracha. All other data is incompatible.

#### 1. Back everything up

Before going any further, back everything up on the server. This includes files
and databases, if an external database like MySQL or PostgreSQL was used.

Store the backup somewhere other than the server, such as your computer's hard
drive. Keep this backup handy, even if the migration appears to be successful.

#### 2. Migrate TinyIB to PostgreSQL

If you are already using PostgreSQL as your TinyIB database, you may skip this step.

Use TinyIB's built in database migration tool to migrate your database to PostgreSQL.
This may require migrating to an intermediate database, depending on which `TINYIB_DBMODE`
is in use.

##### A. Migrate to SQLite or flat file

If your `TINYIB_DBMODE` is set to `sqlite`, `sqlite3` or `flatfile`, skip to part B.

Set `TINYIB_DBMIGRATE` to `sqlite3` and follow the [migration instructions](https://codeberg.org/tslocum/tinyib#migrate).
If `sqlite3` does not work, try `sqlite`. As a last resort, `flatfile` may be used
as the intermediate database. Set any relevant database configuration options
before migrating.

Once you have migrated your database to `sqlite`, `sqlite3` or `flatfile`, proceed to part B.

##### B. Migrate to PDO

Set `TINYIB_DBMIGRATE` to `pdo`, `TINYIB_DBDRIVER` to `pgsql` and follow the [migration instructions](https://codeberg.org/tslocum/tinyib#migrate).
Set any relevant database configuration options to the new PostgreSQL database.
This database will be read and imported by Sriracha.

#### 3. Configure Sriracha to run in import mode

Add the following to your Sriracha `config.yml`, replacing the example values
with your TinyIB PostgreSQL database connection info and table names.

```yaml
# Note: Posting is disabled when running in import mode.
import:
  # Connection info.
  address: "localhost" # Address:Port to connect to the database.
  username: "tinyib"   # Database username.
  password: "hunter2"  # Database password.
  dbname: "tinyib"     # Database name.
  # Table names.
  posts: "dir_posts"   # Required.
  keywords: "keywords" # Optional.
```

#### 4. Start Sriracha and visit the management panel

Log in to the management panel as a super-administrator and follow the
on-screen prompts. After validating the import configuration, you may initiate
a dry run of the import. If the dry run is successful, you may then initiate
the actual import. You will be prompted for a board directory and name. The
`src` and `thumb` directories, containing all of the uploaded files, must exist
in the chosen board directory.

To migrate a single board, and continue running with only one board, copy `src`
and `thumb` to the root directory and leave the board directory field blank.

#### 5. Restart Sriracha in normal mode

Remove the import configuration option from `config.yml` and restart Sriracha
to re-enable posting.

Don't forget to keep the backup handy, even if the migration appears to
be successful.

## Upgrade

[Go to top](#sections)

Administrators may view current version information in the settings page.

### 1. Back everything up

Before going any further, back everything up on the server. This includes files
and PostgreSQL databases.

Store the backup somewhere other than the server, such as your computer's hard
drive. Keep this backup handy, even if the upgrade appears to be successful.

### 2. Download Sriracha

Download the [latest release](https://codeberg.org/tslocum/sriracha/releases) of
Sriracha for your platform.

### 3. Stop Sriracha

Press `Ctrl+C` in the terminal window where Sriracha is running, or send the
`SIGTERM` signal to the Sriracha server process.

### 3. Replace server binary

Replace the old `sriracha` server binary with the new one. If you are using any
plugins, replace all plugin files with updated versions.

### 4. Copy static files

This step is only required if you are running a static file server with Sriracha.
When Sriracha handles all incoming requests, such as when running locally, the
updated static directory is automatically served.

If you are running a static file server, copy all files in the `static` directory
to `/rootdir/static`, replacing `/rootdir` with the server root directory.

### 5. Restart Sriracha

Database upgrades are handled automatically, regardless of the number of releases
between the old and new version. No extra commands need to be run when upgrading.

Verify no error messages are printed when Sriracha starts. If you see the usual
messages indicating Sriracha is running normally, the upgrade is complete.

## Plugins

[Go to top](#sections)

Sriracha supports building and loading plugins via shared library files. Plugins
are not sandboxed in any way. Every plugin has full access to the system. For
this reason, you should only load plugins you personally compiled after inspecting
the source code. Never load an unofficial plugin compiled by someone else.

Official plugins are located in the [plugin](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin)
directory. Plugin API documentation is available via [godoc](https://pkg.go.dev/codeberg.org/tslocum/sriracha#section-documentation).

| Plugin | Description |
| -- | -- |
| BBCode | Format BBCode in post messages. |
| Fortune | Give your posters some good luck (or bad). |
| Password | Require specific passwords to post. |
| Robot9000 | Require post messages to be unique. |
| Wordfilter | Find and replace text in post messages. |

### Instructions

To build a plugin, run the following commands:

```
cd /path/to/sriracha/plugin/fortune
go build -buildmode=plugin
```

This will compile the fortune plugin as `fortune.so`.

To load a plugin, run the following command:

```
sriracha --config=/path/to/config.yml /path/to/fortune.so
```

Multiple plugin paths may be provided. When a directory is provided, all plugins
in the directory are loaded.

### Compatibility

Only plugins built using the same version of Sriracha may be used.

If you attempt to load a plugin and see an error such as:

```
failed to load plugin ./plugin/fortune/fortune.so: plugin.Open("./plugin/fortune/fortune"): plugin was built with a different version of package codeberg.org/tslocum/sriracha
```

The solution is to rebuild all plugins and Sriracha itself.

### Configuration

Plugins may provide configuration options for users to set in the management panel.

The following configuration option types are available:

- Boolean
- Integer
- Float
- Enum
- String
- Board

Boolean options may only have one value. Options of any other type may have one or multiple values.

An example how to implement a plugin with configuration options is available in
the [Fortune](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/fortune/fortune.go) plugin.

### Events

Plugins may subscribe to receive one or more types of events by implementing
the associated event handlers. For instance, a plugin that subscribes to [Post](https://pkg.go.dev/codeberg.org/tslocum/sriracha#Post)
events would implement [PluginWithPost](https://pkg.go.dev/codeberg.org/tslocum/sriracha#PluginWithPost):

```go
type PluginWithPost interface {
	Plugin
	Post(db *Database, post *Post) error
}
```

An example of how to implement a plugin which receives new post events is
available in the [Fortune](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/fortune/fortune.go) plugin.

## Guides

[Go to top](#sections)

This section contains moderator and administrator guides.

### Moderator guide

As a moderator, you may:

- Add bans
- Extend bans
- Approve posts
- Delete posts
- Sticky threads
- Lock threads
- Add news
- Update news

#### Approving posts

If posts require approval before being displayed, or if post reports are enabled,
you will need to periodically review the status page in the management panel.
The status page is the default page shown when you log in. When posts require
moderator approval, they will appear on this status page.

#### Banning IP addresses

Single IP addresses and IP address ranges may be banned. To ban an IP address
range, use a wildcard (*) at the end of the range prefix:

`192.168.1.*`

#### Browsing in mod mode

Mod mode is a tool staff members may use to moderate one or more posts. After
logging into the management panel, viewing any board index page or thread page
normally. Scroll to the bottom of the page and click the delete button. You
will be redirected to the board index page or thread page you were viewing with
mod mode enabled. The following moderation links are shown when mod mode is enabled:

`S L D B D&B`

- S: Sticky thread
- L: Lock thread
- D: Delete post
- B: Ban post author
- D&B: Delete post and ban post author

### Administrator guide

As an administrator, in addition to the moderator capabilities, you may:

- Lift bans
- Add boards
- Update boards
- Add keywords
- Update keywords
- Delete keywords
- Delete news
- Update settings
