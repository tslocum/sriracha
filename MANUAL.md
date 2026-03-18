# Sriracha Manual
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

### Sections

- [**Install**](#install)
- [**Configure**](#configure)
- [**Plugins**](#plugins)
- [**Upgrade**](#upgrade)
- [**Migrate**](#migrate)
- [**Guides**](#guides)

## Install

[Go to top](#sections)

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

#### Native

Download the [latest release](https://codeberg.org/tslocum/sriracha/releases) of
Sriracha for your platform. Only Linux, FreeBSD and macOS are supported.

Linux release archives for `amd64` include all official plugins. To use plugins
on FreeBSD or macOS, or on other CPU architectures, compile Sriracha and its
plugins using the release source code.

#### Docker

To run Sriracha inside a container using [Docker](https://www.docker.com), use an [official image](https://hub.docker.com/r/tslocum/sriracha/tags)
or build an image locally using the included [Dockerfile](https://codeberg.org/tslocum/sriracha/src/branch/main/Dockerfile).

To deploy Sriracha using [Docker Compose](https://docs.docker.com/compose/), use the included
[docker-compose.yml](https://codeberg.org/tslocum/sriracha/src/branch/main/docker-compose.yml).

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

When Sriracha receives a `SIGHUP` signal, all static files are rebuilt.

When Sriracha receives a `SIGINT` or `SIGTERM` signal, new web requests stop
being served, existing web requests are allowed to finish processing, all
pending changes to static files are written to disk and all pending
notifications are sent.

### Root directory

The root directory is where all board directories, attachments, thumbnails and
static HTML pages are located. Sriracha will delete or overwrite files in this
directory whenever data is updated.

While you can add custom files to the root directory, to avoid risk of deletion
you should instead configure your HTTPS server to serve the custom static files
from a separate directory outside of the root directory.

### Board types

Each board is either an imageboard or a forum. The difference between the two
is purely cosmetic. A board's type may be changed at any time.

- Imageboards display attachment thumbnails and truncated messages in index pages.
The first post in a thread has a unique style.
- Forums only display thread subjects in index pages. All posts in a thread
have the same style.

As a general rule, boards which accept image and video attachments should be
imageboards, and boards which do not should be forums.

### Board categories

Categories may be used to organize boards. When at least one category exists, only
categorized boards are displayed in the site index and navigation header. When no
categories exist, all visible boards are displayed.

### Post batching

When a visitor creates a new post, and less than ten seconds have passed since
someone last created a post, static files are not immediately updated.

Once either a full second passes without any new posts being created, or ten
seconds pass, static files are updated and visitors are redirected.

This batching is invisible to visitors and allows the server to handle an influx
of posts without wasting resources writing and immediately overwriting pages.

### Email notifications

Sriracha may optionally allow visitors to subscribe to receive email notifications
when new posts are created. This feature requires an [SMTP](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol) server.

Depending on the configuration of your mail server, you should connect on port
587, 465 or 25, in order from most to least secure.

`mailtls` should be set to `true` if possible, which will connect to the mail
server using a secure connection. If `mailtls` is set to `false`, a plain text
connection is established. However, if the mail server supports the `STARTTLS`
extension, the connection is immediately upgraded to TLS.

`mailinsecure` should be set to `false` if possible, which will require TLS
certificate verification when connecting to the server. The hostname or IP
address used to connect to the mail server must be listed in the certificate.
If `mailinsecure` is set to `true`, TLS certificate verification is skipped.

`mailauth` should be set to `challenge` if possible, which will enable the [CRAM-MD5](https://en.wikipedia.org/wiki/CRAM-MD5)
challenge-response authentication mechanism. `mailauth` may instead be set to
`plain` to authenticate using a plain text password. `mailauth` may also be set
to `none` to skip authentication entirely.

Notifications will contain clickable links when the `Site Home` setting is
configured as a URL (e.g. `https://example.com/`) rather than a relative path.

Sriracha does not read any incoming emails. Replies via email are ignored.
Because of this, you should bounce emails sent to the notification address.

If you only want to allow subscriptions from visitors with specific email
address domains, set `maildomains` to a reguar expression:

```yaml
maildomains: '^(example\.com|example2\.com|example3\.com)$'
```

### Custom templates

Sriracha supports overriding official templates with custom templates. This
section is a short guide on how to use custom templates.

Sriracha template files have the extension `.gohtml` and are written in the
[Go HTML template](https://pkg.go.dev/html/template) language. All official
templates are included in release archives.

To override a template, create a directory where the custom template files will be stored.
Custom template files should be stored somewhere outside of the root directory.
Set the `template` option in `config.yml` to the newly created directory path.

Create a custom template file in the new directory with the same name as the
official template you wish to override. Copy the official template file contents
into the new custom template file, then apply any desired changes.

Template files with the prefix `extra_` are normally blank and are provided for
convenience. These templates do not change between each version of Sriracha, so
they do not require any maintenance. Whenever possible, you should only override
templates with the `extra_` prefix.

All other template files may still be overridden to customize Sriracha, but the
custom templates will require maintenance each time Sriracha is upgraded.
Official Sriracha templates change between each version, and these changes must
be copied to any custom templates overriding them before starting Sriracha.

To apply custom template changes when Sriracha is running normally, restart
Sriracha. When Sriracha is running in development mode, template changes are
applied automatically whenever template files are modified. Pass the flag
`--dev` when starting Sriracha to run in development mode.

All official templates are included in release archives to facilitate
customizing and overriding individual templates. If you ignore the instructions
above and use the entire official template directory as a custom template
directory, please don't ask for support.

Support is not available for Sriracha installations using custom templates.

Support is not available for creating or modifying custom template files.

### Custom pages

Pages may access the database via the following read-only methods:

- [Board](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Board)
  - `BoardByID(id int) *Board`
  - `BoardByDir(dir string) *Board`
  - `UniqueUserPosts(b *Board) int`
  - `AllBoards() []*Board`
- [News](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#News)
  - `NewsByID(id int) *News`
  - `AllNews(onlyPublished bool) []*News`
- [Page](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Page)
  - `PageByID(id int) *Page`
  - `PageByPath(path string) *Page`
  - `AllPages() []*Page`
- [Post](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Post)
  - `AllThreads(board *Board, moderated bool) [][2]int`
  - `AllPostsInThread(postID int, moderated bool) []*Post`
  - `AllReplies(threadID int, limit int, moderated bool) []*Post`
  - `PendingPosts() []*Post`
  - `PostByID(postID int) *Post`
  - `PostsByIP(hash string) []*Post`
  - `PostsByFileHash(hash string, filterBoard *Board) []*Post`
  - `PostByField(b *Board, field string, value any) *Post`
  - `LastPostByIP(board *Board, ip string) *Post`
  - `ReplyCount(threadID int) int`

For example, the following custom page will render all moderated posts in the
board with ID #7 by printing their ID, subject and message:

```gohtml
{{$onlyShowModerated := true}}
{{$board := BoardByID 7}}
{{$threads := AllThreads $board $onlyShowModerated}}
<hr>
Found {{len $threads}} threads.
{{range $i, $thread := $threads}}
    {{$threadID := index $thread 0}}
    {{$threadReplyCount := index $thread 1}}
    <hr>
    Thread No.{{$threadID}} (Replies: {{$threadReplyCount}})
    {{range $post := AllPostsInThread $threadID $onlyShowModerated}}
        <br><br>
        ID: {{$post.ID}}<br>
        Subject: {{$post.Subject}}<br>
        Message {{$post.Message | HTML}}
    {{end}}
{{end}}
<hr>
```

### Locales

The following `locale` configuration options are available:

| Language | Locale |
| --       | --     |
| English  | `en`   |
| Albanian | `sq`   |
| Dutch    | `nl`   |
| Finnish  | `fi`   |
| Russian  | `ru`   |

Help translate Sriracha into additional languages [online](https://translate.codeberg.org/projects/sriracha/sriracha/).

### Example configuration (config.yml)

```yaml
# Interface language. See locale directory for full list.
locale: "en"

# Directory where board files are written to.
root: "/home/sriracha/public_html"

# Hostname:Port to listen for HTTP connections on.
serve: "localhost:8080"

# Client IP address header. Must be set when running behind a reverse proxy.
# When running behind CloudFlare, use CF-Connecting-IP. When running without
# a proxy, leave blank.
#header: "X-Forwarded-For"

# Hash algorithm. Supported algorithms are sha-3 (recommended) and sha-2. Must not change once set.
algorithm: "sha-3"

# Long random string of text used when one-way hashing data. Must not change once set.
saltdata: "CHANGEME_Random_Data_Here_1"

# Long random string of text used when two-way hashing data. Must not change once set.
saltpass: "CHANGEME_Random_Data_Here_2"

# Long random string of text used when generating secure tripcodes. Must not change once set.
salttrip: "CHANGEME_Random_Data_Here_3"

# Hostname:Port to connect to the database.
address: "localhost:5432"

# Database username.
username: "sriracha"

# Database password.
password: "hunter2"

# Database name.
dbname: "sriracha"

# Database connection URL. Allows specifying additional connection options.
# This option supercedes the address, username, password and dbname options.
# You probably don't need this. Configure the database options above instead.
# See https://pkg.go.dev/github.com/jackc/pgx/v5@v5.7.4/pgconn#ParseConfig
#dburl: "postgresql://sriracha:hunter2@localhost:5432/sriracha"

# SMTP mail server configuration. When configured, visitors may subscribe to
# to receive email notifications when new posts are created. To disable email
# notifications, leave mailaddress blank. To allow subscriptions using any
# email address domain, leave maildomains blank.
#mailaddress:  ""          # SMTP server Hostname:Port.
#mailtls:      true        # Whether TLS is used to connect to the server.
#mailinsecure: false       # Whether TLS certificate verification is skipped.
#mailusername: ""          # SMTP server username.
#mailpassword: ""          # SMTP server password.
#mailauth:     "challenge" # SMTP server authentication mechanism. Format: challenge / plain / none
#mailfrom:     ""          # Notification "From" email address.
#mailreplyto:  ""          # Notification "Reply-To" email address.
#maildomains:  ""          # Regular expression specifying allowed email address domains.

# Notification batch durations. These options only apply when a mail server is configured.
#mentions:      60   # Duration (in minutes) mention notifications are batched together.
#notifications: 1440 # Duration (in minutes) all other notifications are batched together.

# Access required to perform an action. Default values for all actions are listed below.
#
# Format: mod / admin / super-admin / disable (disallow all roles)
#access:
#  ban.add:         "mod"
#  ban.shorten:     "admin"
#  ban.lengthen:    "mod"
#  ban.delete:      "admin"
#  banfile.add:     "mod"
#  banfile.delete:  "admin"
#  banner.add:      "admin"
#  banner.update:   "admin"
#  banner.delete:   "super-admin"
#  board.add:       "admin"
#  board.update:    "admin"
#  board.delete:    "super-admin"
#  category.add:    "admin"
#  category.update: "admin"
#  category.delete: "super-admin"
#  keyword.add:     "admin"
#  keyword.update:  "admin"
#  keyword.delete:  "admin"
#  page.add:        "admin"
#  page.update:     "admin"
#  page.delete:     "admin"
#  post.sticky:     "mod"
#  post.lock:       "mod"
#  post.move:       "mod"
#  post.delete:     "mod"

# Whether identifier hashes are enabled. Identifier hashes are generated based
# on IP hashes. When enabled, staff may view and delete all posts created by an
# IP address, and boards may optionally display identifier hashes to visitors.
#identifiers: false

# Custom template directory. Leave blank to use official templates. Template
# files in this directory will override official templates of the same name.
#template: "/home/sriracha/template"

# Supported upload file types. Specify a file extension and a MIME type to
# enable uploading files of that type. You may specify an image to use as the
# thumbnail for all uploads of that type, or 'none' to not create a thumbnail.
# Otherwise, thumbnails are generated automatically based on the uploaded file.
# To generate thumbnails for videos or SVG images, ffmpeg must be installed.
# Note: Opus audio files are detected as audio/ogg.
#
# Format: ext mime thumbnail
uploads:
  - "jpg image/jpeg"
  - "jpg image/pjpeg"
  - "png image/png"
  - "gif image/gif"
  - "svg image/svg+xml"
  - "wav audio/wav media.png"
  - "wav audio/wave media.png"
  - "wav audio/x-wav media.png"
  - "aac audio/aac media.png"
  - "ogg audio/ogg media.png"
  - "flac audio/flac media.png"
  - "mp3 audio/mp3 media.png"
  - "mp3 audio/mpeg media.png"
  - "mp4 audio/mp4 media.png"
  - "mp4 video/mp4"
  - "webm audio/webm media.png"
  - "webm video/webm"
  - "swf application/x-shockwave-flash swf.png"
```

### Example reverse proxy using caddy (Caddyfile)

```caddyfile
# Serve https://zoopz.org and https://www.zoopz.org
zoopz.org, www.zoopz.org {
  # Enable zstd and gzip compression.
  encode zstd gzip

  # Revalidate HTML files.
  header * ?Cache-Control "public, no-cache"

  # Cache static files.
  @cachedFiles {
    path *.aac *.avi *.css *.flac *.gif *.ico *.jpg *.js *.mp3 *.mp4 *.ogg *.opus *.png *.svg *.swf *.wasm *.wav *.webm *.webp *.woff
  }
  header @cachedFiles Cache-Control "public, max-age=1209600, immutable"

  # Forward /sriracha requests to Sriracha.
  reverse_proxy /sriracha* http://localhost:8080

  # Serve root directory.
  root * /home/sriracha/public_html
  file_server
}
```

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
| [BBCode](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/bbcode/bbcode.go) | Format BBCode in post messages. |
| [Fortune](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/fortune/fortune.go) | Give your visitors some good luck (or bad). |
| [IRC](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/irc/irc.go) | Send server event notifications. |
| [Password](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/password/password.go) | Require specific passwords to post. |
| [Robot9000](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/robot9000/robot9000.go) | Require post messages to be unique. |
| [Statistics](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/statistics/statistics.go) | View statistics for each board. |
| [Wordfilter](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/wordfilter/wordfilter.go) | Find and replace text in post messages. |

### Using plugins

To build a plugin, run the following commands:

```bash
cd /path/to/sriracha/plugin/fortune
go build -buildmode=plugin
```

This will compile the fortune plugin as `fortune.so`.

To load a plugin, run the following command:

```bash
sriracha --config=/path/to/config.yml /path/to/fortune.so
```

Multiple plugin paths may be provided. When a directory is provided, all plugins
in the directory are loaded.

### Plugin compatibility

Only plugins built using the same version of Sriracha may be used.

If you attempt to load a plugin and see an error such as:

```
failed to load plugin ./plugin/fortune/fortune.so: plugin.Open("./plugin/fortune/fortune"): plugin was built with a different version of package codeberg.org/tslocum/sriracha
```

The solution is to rebuild all plugins and Sriracha itself.

### Plugin interface

All plugins must implement the Plugin interface:

```go
// Plugin describes the required methods for a plugin.
type Plugin interface {
  // About returns the plugin description.
  About() string
}
```

### Plugin configuration

Plugins may optionally specify any number of configuration options:

```go
// PluginWithConfig describes the required methods for a plugin with configuration options.
type PluginWithConfig interface {
  Config() []PluginConfig
}
```

These options may be viewed and modified in the management panel.

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

### Plugin events

Plugins may subscribe to receive one or more types of events by implementing
the associated event handlers.

For example, a plugin subscribing to [Post](https://pkg.go.dev/codeberg.org/tslocum/sriracha#Post)
events would implement [PluginWithPost](https://pkg.go.dev/codeberg.org/tslocum/sriracha#PluginWithPost).

When a plugin handles an event, a reference to the database is provided. This reference
is only valid during the event handler call.

#### Update event

Update events are sent when a configuration option is modified. Update events
are also sent for each configuration option when the server initializes.

```go
// PluginWithUpdate describes the required methods for a plugin subscribing to configuration updates.
type PluginWithUpdate interface {
  Update(db sriracha.DB, key string) error
}
```

#### Rules event

Rules returns text informing visitors of available features and functionality.
This text is displayed below the post form in board index and thread pages.

```go
// PluginWithRules describes the required methods for a plugin with text displayed below the post form.
type PluginWithRules interface {
  Rules(db sriracha.DB, board *Board) (template.HTML, error)
}
```

#### Attach event

Attach events are sent when a file is attached to a post. FileOriginal contains
the original file name and FileMIME contains the detected MIME type. When a file
attachment is handled, return true to stop propagating events to other plugins.

```go
// PluginWithAttach describes the required methods for a plugin subscribing to attach events.
type PluginWithAttach interface {
  Attach(db sriracha.DB, post *Post, file multipart.File) (handled bool, err error)
}
```

#### Embed event

Embed events are sent when a URL is embedded in a post. When an embed URL is
handled, return true to stop propagating events to other plugins.

```go
// PluginWithEmbed describes the required methods for a plugin subscribing to embed events.
type PluginWithEmbed interface {
  Embed(db sriracha.DB, post *Post, embedURL string) (handled bool, err error)
}
```

#### Post event

Post events are sent when a new post is being created. Message is the only
HTML-escaped field. Newlines are conveted into line break tags after all
plugins have finished processing the post.

```go
// PluginWithPost describes the required methods for a plugin subscribing to post events.
type PluginWithPost interface {
  Post(db sriracha.DB, post *Post) error
}
```

#### Insert event

Insert events are sent after Post events have been processed, before a new post
is inserted. The post may not be modified during this event. Modify new posts
during a Post event instead. Return an error to cancel the post, or nil to
continue processing.

```go
// PluginWithInsert describes the required methods for a plugin subscribing to insert events.
type PluginWithInsert interface {
  Insert(db sriracha.DB, post *Post) error
}
```

#### Create event

Create events are sent when a new post is created and inserted into the database,
after Post and Insert events have been processed. The post may not be modified
during this event. Modify posts during a Post event instead.

```go
// PluginWithCreate describes the required methods for a plugin subscribing to create events.
type PluginWithCreate interface {
  Create(db sriracha.DB, post *Post) error
}
```

#### Report event

Report events are sent when a post is reported.

```go
// PluginWithReport describes the required methods for a plugin subscribing to report events.
type PluginWithReport interface {
  Report(db sriracha.DB, post *Post) error
}
```

#### Audit event

Audit events are sent when a new message is added to the audit log.
Based on the source of the event, user is "system", "admin" or "mod".

```go
// PluginWithAudit describes the required methods for a plugin subscribing to audit events.
type PluginWithAudit interface {
  Audit(db sriracha.DB, user string, action string, info string) error
}
```

#### Serve event

Serve handles plugin web requests. Only administrators and super-administrators
may access this page. When serving HTML responses, return the HTML and a nil
error. When serving any other content type, set the Conent-Type header, write
to the `http.ResponseWriter` directly and return a blank string.

```go
// PluginWithServe describes the required methods for a plugin with a web interface.
type PluginWithServe interface {
  Serve(db sriracha.DB, a *Account, w http.ResponseWriter, r *http.Request) (template.HTML, error)
}
```

## Upgrade

[Go to top](#sections)

Administrators may view current version information in the settings page.

### 1. Back everything up

Before going any further, back everything up on the server. This includes files
and PostgreSQL databases.

Store the backup somewhere other than the server, such as your computer's hard
drive. Keep this backup handy, even if the upgrade appears successful.

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

When running a static file server, copy all files in the `static` directory
to `/rootdir/static`, replacing `/rootdir` with the server root directory.

When Sriracha handles all incoming requests, such as when running locally, the
updated static directory is served automatically.

### 5. Restart Sriracha

Database upgrades are handled automatically, regardless of the number of releases
between the old and new version.

Static HTML files (news, overboard, board indexes and threads) are rebuilt
automatically when Sriracha is upgraded.

Verify no error messages are printed when Sriracha starts. If you see the usual
messages indicating Sriracha is running normally, the upgrade is complete.

## Migrate

[Go to top](#sections)

Sriracha supports exporting and importing posts via [SQLite](https://sqlite.org) database files.

### Export posts

To export posts, start Sriracha with the `--export` flag and specify where the
export ZIP archive will be written:

```bash
sriracha --export=/home/sriracha/export.zip
```

Attachment files are not included within the export. To import posts later, you
will also need a copy of the `src` and `thumb` directories of each board.

### Import posts

To import posts, start Sriracha with the `--import` flag and specify the path
of an export ZIP archive or SQLite database file:

```bash
sriracha --import=/home/sriracha/export.zip
```

Note: Posting is disabled when running in import mode.

Attachment files are not included within the export. To import posts, you will
also need a copy of the `src` and `thumb` directories of each board.

Log in to the Sriracha management panel as a super-administrator to complete the import.

### Import posts from TinyIB

Sriracha supports importing posts from [TinyIB](https://codeberg.org/tslocum/tinyib). Differences between Sriracha and TinyIB:

- **Only PostgreSQL is supported**
  - Sriracha only supports the [PostgreSQL](https://www.postgresql.org) database system.
- **Account roles have different capabilities**
  - See the administrator and moderator [guides](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#guides)
for a list of role capabilities.
- **Single auto-increment post ID**
  - Sriracha uses one auto-incrementing post ID for all boards. When only one board
is migrated, post IDs will remain unchanged. Migrating more than one board will
cause post IDs to be renumbered. Reference links inside post messages will be
updated, but external links to old res pages will break.
- **IP address and file hashes are incompatible**
  - Sriracha hashes IP addresses and files by generating a [SHA384](https://en.wikipedia.org/wiki/SHA-2)
hash of the data. TinyIB hashes IP addresses and files using [crypt](https://www.php.net/manual/en/function.crypt.php).
Because of this, posts will have their IP address field blanked.
File hashes are recalculated and corrected during import.
- **All keywords are regular expressions**
  - Sriracha keywords are always [regular expressions](https://en.wikipedia.org/wiki/Regular_expression).
- **Licensed under GNU LGPL**
  - Sriracha is licensed under [GNU LGPL](https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE).
If you modify the source code of this application, you must share the full
source code of your changes publicly for free. You may, however, link with this
application using proprietary shared libraries, so long as the base application
(Sriracha) remains unmodified. If your only changes are to create proprietary
shared libraries, and these librarires would work with other installations of
Sriracha because you did not make any modifications to Sriracha's source code,
then you do not need to release the source code of your shared libraries.
If you run an unmodified official release [archive](https://codeberg.org/tslocum/sriracha/releases) or
[image](https://hub.docker.com/r/tslocum/sriracha), or if you compile Sriracha
using only the unmodified source code of an official release, then you do not
need to share any source code.

#### 1. Back everything up

Before going any further, back everything up on the server. This includes files
and databases, if an external database like MySQL or PostgreSQL was used.

Store the backup somewhere other than the server, such as your computer's hard
drive. Keep this backup handy, even if the migration appears successful.

#### 2. Migrate TinyIB to SQLite

If you are already using SQLite as your TinyIB database, you may skip this step.

Use TinyIB's built in database migration tool to migrate your database to SQLite.
Set `TINYIB_DBMIGRATE` to `sqlite3` and follow the [migration instructions](https://codeberg.org/tslocum/tinyib#migrate).

If `sqlite3` does not work, try `sqlite`. If neither work, you will need to
enable SQLite in the configuration of your PHP installation.

Once you have migrated your TinyIB database to `sqlite3` or `sqlite`, proceed to the next step.

#### 3. Start Sriracha in import mode

Start sriracha with the --import flag and specify the path to a TinyIB database file:

```bash
sriracha --import=/home/sriracha/tinyib.db
```

Note: Posting is disabled when running in import mode.

The `src` and `thumb` directories, which contain post attachment files, must be
copied from TinyIB to the Sriracha root directory.

#### 4. Visit the management panel

Log in to the Sriracha management panel as a super-administrator.

After validating the import configuration, you will be prompted for which board
to import posts into. You may then start a dry run of the import.

To migrate a single board installation, create a board with a blank directory
and copy `src` and `thumb` to the root directory.

When performing a dry run of the import, Sriracha will verify the presence of
all expected attachment files.

If the dry run is successful, you may then initiate the actual import.

#### 5. Restart Sriracha in normal mode

Restart Sriracha without the --import flag to re-enable posting.

Don't forget to keep the backup handy, even if the migration appears to
be successful.

### Import posts from other software

Sriracha is capable of importing posts from any software, provided you or
someone else export the data in a compatible format.

Sriracha supports importing post data via SQLite database files containing one table:

```sql
CREATE TABLE post (
  id           INTEGER PRIMARY KEY, -- ID number.
  parent       INTEGER NOT NULL,    -- Parent post ID. Thread posts have their parent set to 0.
  timestamp    INTEGER NOT NULL,    -- Unix timestamp (in seconds) the post was created.
  bumped       INTEGER NOT NULL,    -- Unix timestamp (in seconds) the post was last bumped. Only applies to threads.
  name         TEXT NOT NULL,       -- Poster name.
  tripcode     TEXT NOT NULL,       -- Poster tripcode. Exclude first '!' character.
  email        TEXT NOT NULL,       -- Poster email address. Does not need to be a valid email address.
  nameblock    TEXT NOT NULL,       -- HTML-formatted field containing the poster name, tripcode, capcode, identifier and post date/time.
  subject      TEXT NOT NULL,       -- Post subject.
  message      TEXT NOT NULL,       -- HTML-formatted post message.
  file         TEXT NOT NULL,       -- Attachment file name. Located in board src directory.
  filemime     TEXT NOT NULL,       -- Attachment MIME type.
  filehash     TEXT NOT NULL,       -- Hash of attachment file. When exporting/importing posts, this is only used for embed attachments.
  fileoriginal TEXT NOT NULL,       -- Original file name.
  filesize     INTEGER NOT NULL,    -- Size (in bytes) of attachment.
  filewidth    INTEGER NOT NULL,    -- Width (in pixels) of attachment.
  fileheight   INTEGER NOT NULL,    -- Height (in pixels) of attachment.
  thumb        TEXT NOT NULL,       -- Thumbnail file name. Located in board thumb directory.
  thumbwidth   INTEGER NOT NULL,    -- Width (in pixels) of thumbnail.
  thumbheight  INTEGER NOT NULL,    -- Height (in pixels) of attachment.
  stickied     INTEGER NOT NULL,    -- 0: Unstickied. 1: Stickied.
  locked       INTEGER NOT NULL     -- 0: Unlocked. 1: Locked.
);
```

All fields are plain text except where noted.

You will most likely want to leave the nameblock field of each post blank.
Sriracha will then rebuild each post's nameblock during import.

The above schema describes posts with file attachments. Posts with embed
attachments use the same fields with the following differences:

- `file` contains HTML which will be displayed when the embed is expanded instead of a file name.
- `filehash` contains embed information in the format `e ServiceName Title of Embedded Content` instead of a hash.
- `fileoriginal` contains the URL of the embedded content instead of a file name.

The following fields may be left blank and will be filled during import:

- When `bumped` is less than or equal to zero, it will be set to `timestamp`.
- When `nameblock` is blank, it will be rebuilt based on other fields.
- When `filemime` is blank, and the post has a file attachment, it will be set to the detected MIME type.
- When `filehash` is blank, and the post has a file attachment, it will be set to a newly calculated file hash.
- When `filesize` is blank, and the post has a file attachment, it will be set to the size of the file.
- When `filewidth` is less than or equal to zero, and the post has a JPG, PNG or GIF attachment, it will be set to the width of the attachment.
- When `fileheight` is less than or equal to zero, and the post has a JPG, PNG or GIF attachment, it will be set to the height of the attachment.
- When `thumbwidth` is less than or equal to zero, and the post has a JPG, PNG or GIF thumbnail, it will be set to the width of the thumbnail.
- When `thumbheight` is less than or equal to zero, and the post has a JPG, PNG or GIF thumbnail, it will be set to the height of the thumbnail.

#### Example post with file attachment

```json
{
  "id": 3,
  "parent": 2,
  "timestamp": 1623399789,
  "bumped": 1623399789,
  "name": "Maecenas",
  "tripcode": "",
  "email": "",
  "nameblock": "<span class=\"postername\">Maecenas</span> 2021/06/11<wbr>(Fri)<wbr>01:23:09",
  "subject": "",
  "message": "Integer neque lacus, posuere ac massa et, feugiat rhoncus elit. Phasellus aliquet turpis a magna vehicula placerat. Praesent finibus massa enim, ac vestibulum metus dignissim ac. Proin cursus nisi et dui facilisis efficitur. Integer blandit est ac turpis laoreet, vitae interdum orci rhoncus. Phasellus faucibus vitae massa id rhoncus. Nam est lorem, ornare eu magna quis, tristique elementum sem. Nunc aliquet metus nunc, quis rutrum leo vehicula a. Morbi ut augue erat. Nam nec tempor enim. Morbi quis vestibulum libero. Sed molestie est sapien. Morbi tellus sapien, facilisis eget eleifend a, dignissim id justo. Maecenas ut tortor nibh.",
  "file": "1623399789642.jpg",
  "filemime": "image/jpeg",
  "filehash": "",
  "fileoriginal": "kaikaku.jpg",
  "filesize": 17767,
  "filewidth": 474,
  "fileheight": 353,
  "thumb": "1623399789642s.jpg",
  "thumbwidth": 250,
  "thumbheight": 186,
  "stickied": 0,
  "locked": 0
}
```

> [>>3](https://sriracha.rocket9labs.com/img/res/2.html#3)

#### Example post with embed attachment

```json
{
  "id": 7,
  "parent": 2,
  "timestamp": 1623401066,
  "bumped": 1623401066,
  "name": "Nullam",
  "tripcode": "",
  "email": "",
  "nameblock": "<span class=\"postername\">Nullam</span> 2021/06/11<wbr>(Fri)<wbr>01:44:26",
  "subject": "",
  "message": "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Proin interdum, eros in mollis placerat, risus lorem laoreet odio, sit amet faucibus erat libero ut risus. Nullam quis ipsum eu libero convallis aliquet mattis sollicitudin nisi. Morbi nec vulputate nisi. Maecenas sagittis sodales efficitur. Maecenas volutpat ipsum quis est congue, in faucibus ante efficitur. Quisque vestibulum mattis metus eu aliquet. Vestibulum eu tincidunt arcu.\n<br>\n<br>In non aliquam dui, sit amet ullamcorper magna. Nunc id enim ac felis volutpat varius. Aenean auctor orci quam, et vehicula nunc placerat id. Duis quis purus eu urna semper facilisis nec ut sapien. Curabitur porta in diam non volutpat. Proin congue nibh at commodo mattis. Quisque pharetra arcu at nulla finibus consectetur. Ut efficitur vestibulum quam vitae molestie. Ut rhoncus rutrum dignissim.\n<br>\n<br>Sed sed tellus pulvinar, hendrerit orci nec, elementum dui. Cras at neque risus. Fusce elit nisl, sollicitudin eget pulvinar eu, pretium a neque. Donec eleifend convallis tortor at pharetra. Vestibulum nec nibh diam. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Aliquam quis mauris quis nisi ornare commodo. Nunc malesuada sem vel diam ullamcorper sodales. Maecenas tempus, tortor ac molestie gravida, neque nunc egestas lorem, elementum commodo velit lorem eget dolor. Sed consequat condimentum velit vehicula blandit. Proin ut ipsum sit amet elit consectetur sollicitudin. Cras lacinia vehicula orci vitae fermentum.",
  "file": "<iframe width=\"200\" height=\"113\" src=\"//www.youtube.com/embed/6_NeqMAAsBk?feature=oembed\" frameborder=\"0\" allow=\"accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture\" allowfullscreen></iframe>",
  "filemime": "",
  "filehash": "e YouTube 10/20/1984",
  "fileoriginal": "https://www.youtube.com/watch?v=6_NeqMAAsBk",
  "filesize": 0,
  "filewidth": 480,
  "fileheight": 360,
  "thumb": "1623401066680.jpg",
  "thumbwidth": 250,
  "thumbheight": 188,
  "stickied": 0,
  "locked": 0
}
```

> [>>7](https://sriracha.rocket9labs.com/img/res/2.html#7)

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

When there are no pending moderation requests, the status page will automatically
refresh after five minutes have passed.

#### Banning IP addresses

Single IP addresses and IP address ranges may be banned. To ban an IP address
range, use a wildcard (*) at the end of the range prefix:

`192.168.1.*`

#### Browsing in mod mode

Mod mode is a tool staff members may use to moderate one or more posts.
When browsing in mod mode, the following moderation links are displayed:

`V M S L D B D&B`

- V: View posts by author
- M: Move thread
- S: Sticky thread
- L: Lock thread
- D: Delete post
- B: Ban post author
- D&B: Delete post and ban post author

A shortcut for accessing mod mode is available when logged in. View any index
or thread page normally, scroll to the bottom of the page and click the delete
button. If you are logged in to a staff account, you will be redirected to the
page you were just viewing with mod mode enabled.

### Administrator guide

As an administrator, in addition to all moderator capabilities, you may:

- Lift bans
- Add boards
- Update boards
- Add keywords
- Update keywords
- Delete keywords
- Delete news
- Update settings

### Super-administrator guide

As a super-administrator, you have unrestricted access.

#### Rebuild name blocks

Each post has a `nameblock` field containing the poster's name, tripcode and
identifier, as well as the date/time the post was created. Super-administrators
may rebuild all `nameblock` fields by visiting `/sriracha/?rebuildNameblocks`.

#### Verify IP address resolution

To support banning visitors, Sriracha must be able to resolve remote IP addresses,
which is only possible when the header server option is configured correctly.
Super-administrators may verify remote IP address resolution by visiting `/sriracha/?remoteAddress`.

#### Scan for unexpected files

While Sriracha always attempts to clean up any files related to posts, it is
possible for files which are not attached to any posts to accumulate in board
directories. Super-administrators may scan for any unexpected files by visiting `/sriracha/?scanFiles`.
