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

A [quick start guide](https://codeberg.org/tslocum/sriracha/src/branch/main/QUICKSTART.md) is available.

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
Sriracha for your platform. Only Linux, FreeBSD, OpenBSD and macOS are supported.

#### Docker

To run Sriracha inside a container using [Docker](https://www.docker.com), use an [official image](https://hub.docker.com/r/tslocum/sriracha/tags)
or build an image locally using the included [Dockerfile](https://codeberg.org/tslocum/sriracha/src/branch/main/Dockerfile).

To deploy Sriracha using [Docker Compose](https://docs.docker.com/compose/), use the included
[docker-compose.yml](https://codeberg.org/tslocum/sriracha/src/branch/main/docker-compose.yml).

#### Build from source

Install [Git](https://git-scm.com), the [Go compiler](https://go.dev) and [goreleaser](https://goreleaser.com):

```
go install github.com/goreleaser/goreleaser/v2@latest
```

Clone the Sriracha source code repository:

```
git clone https://codeberg.org/tslocum/sriracha.git
```

Change directories to the Sriracha source code:

```
cd sriracha
```

**Note:** Only tagged official Sriracha releases are supported. Base any modifications on the latest version tag.

Check out the latest release (replace v1.0.0 with the [latest tag](https://codeberg.org/tslocum/sriracha/tags)):

```
git checkout v1.0.0
```

Build Sriracha release archives:

```
goreleaser --clean --snapshot
```

Sriracha release archives will then be available in the `dist` directory.

##### Unsupported platform

Sriracha supports Linux and other Unix based platforms. You should use a
supported platform to run Sriracha. If that is not possible, and Docker is
available for your platform, you should use Docker to run Sriracha.

If you want to run Sriracha on a platform not supported by Docker, try following
the compilation instructions above first.

If Sriracha fails to compile, try building Sriracha without import/export functionality:

```
go build -tags nosqlite ./cmd/sriracha/
```

**Note:** If you want to import or export posts later, you will need to perform
the operation on a supported platform or in a Docker container instead.

If you are still having trouble running Sriracha on an unsupported platform,
please open an issue or create a thread on /help/ explaining the problem.

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

When running behind a frontend, the `header` option must be set appropriately.
Most web servers use `X-Forwarded-For` to specify the client IP address.

Only requests to `/sriracha/*` need to be served by Sriracha. After copying
`static` to the root directory, you may handle all requests except `/sriracha/*`
using a static file server.

When starting Sriracha for the first time, visit the management panel at
`/sriracha/` and log in to the default super-administrator account by entering
`admin` as the username and the password. Once you have logged in, visit the
accounts page and change your username and password.

When Sriracha receives a `SIGHUP` signal, all static files are rebuilt and, if
HTTPS is enabled, certificate files are reloaded.

When Sriracha receives a `SIGINT` or `SIGTERM` signal, new web requests stop
being served, existing web requests are allowed to finish processing, all
pending changes to static files are written to disk and all pending
notifications are sent.

### HTTPS

Sriracha supports serving [HTTPS](https://en.wikipedia.org/wiki/HTTPS) requests
by specifying a certificate and private key pair.

However, you should run a frontend such as [caddy](https://caddyserver.com) with
Sriracha instead, which listens for HTTPS requests and forwards them to Sriracha
as plain [HTTP](https://en.wikipedia.org/wiki/HTTP).

### HTTP/2

[HTTP/2](https://en.wikipedia.org/wiki/HTTP/2) requests are supported. You should use
HTTP/2 instead of [HTTP/1](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/Evolution_of_HTTP)
whenever possible, because HTTP/1 has [flaws](https://portswigger.net/research/http-desync-attacks-request-smuggling-reborn)
which allow [request hijacking](https://www.youtube.com/watch?v=FJbuAyxTTWc).

[HTTP/2 cleartext](https://en.wikipedia.org/wiki/HTTP/2#Encryption) (h2c) is also supported.
You should use h2c when forwarding requests from a frontend to Sriracha,
because encryption is unnecessary for internal connections. If you are running
a frontend with Sriracha, you should also enable `rejecthttp1`.

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

### Stripping metadata

Enable the 'Strip Metadata' option in Settings to strip many types of metadata
(including [Exif](https://en.wikipedia.org/wiki/Exif) tags) from attachments.

This feature requires the installation of [ExifTool](https://exiftool.org),
a free and open source program which handles the manipulation of metadata.

ExifTool will strip all recognized metadata, including potentially helpful metadata,
which may reduce rendering quality. File corruption is also possible.

### Post batching

When a visitor creates a new post, and less than ten seconds have passed since
someone last created a post, static files are not immediately updated.

Once either a full second passes without any new posts being created, or ten
seconds pass, static files are updated and visitors are redirected.

Post batching conserves system resources and is invisible to visitors.

### Embed services

Sriracha supports embedding remote content via [oEmbed](https://en.wikipedia.org/wiki/OEmbed).
When a URL is embedded within a post, Sriracha will make an oEmbed request to
the first service listed. If the first embed service does not recognize the
embed URL, the second service is tried, and so on.

Manage the available embed services by visiting the 'Settings' page.
Embed services may then be enabled per-board, just like upload file types.

### Two-factor authentication

Staff may add up to five [TOTP](https://en.wikipedia.org/wiki/Time-based_one-time_password)
two-factor authentication devices.

A phone or tablet may be used as a two-factor authentication device by
installing a free and open source TOTP authentication application.

When at least one two-factor device has been added, logging in will require
entering a 2FA passcode after entering the correct username and password.

Ideally, the passcodes are generated using a completely separate device which
is never used to access the management panel.

In practice, generating codes using a phone or other secondary device provides
reasonable security.

Generating and validating two-factor authentication passcodes requires accurate
timekeeping. Passcodes will fail validation when the time difference between the
server and a device is greater than a minute and a half.

#### Account recovery

If you lose access to all of your 2FA devices, ask a super-administrator to
reset your password. All existing 2FA devices will be removed from your account.
If you are the only super-administrator, use the `--recover` flag instead.

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

Sriracha supports overriding official templates with custom templates.
This section is a guide on how to use custom templates.

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

Release archives include all official template files in order to facilitate
copying and customizing individual templates. If you use the entire directory
to override every template, there will be weeping and gnashing of teeth.

Support is not available for Sriracha installations using custom templates.

Support is not available for creating or modifying custom template files.

#### Template locations

```
extra_navbar.gohtml [board1] [board2]                                     extra_adminbar.gohtml [Search] [News] [Manage]

extra_header.gohtml

                                                     Banner Image
                                                         Name
                                                   extra_logo.gohtml

 ----------------------------------------------------------------------------------------------------------------------

                                                     Page Content

 ----------------------------------------------------------------------------------------------------------------------

                                                  extra_footer.gohtml
                                                     - Sriracha -
```

#### Troubleshooting template errors

If you encounter either of the following errors:

```
failed to parse template files: failed to parse custom template file <file.gohtml>: template: <file.gohtml>:<line>: <error message>
```

```
failed to validate templates: failed to execute custom template <file.gohtml>: template: <file.gohtml>:<line>:<column>: executing <file.gohtml> at <token>: <error message>
```

A custom template file has failed validation. This may be resolved by fixing
the custom template or by removing it from the custom template directory.

If you are not using any custom templates and you encounter either of the following errors:

```
failed to parse template files: failed to parse official template file <file.gohtml>: template: <file.gohtml>:<line>: <error message>
```

```
failed to validate templates: failed to execute official template <file.gohtml>: template: <file.gohtml>:<line>:<column>: executing <file.gohtml> at <token>: <error message>
```

An official template file has failed validation. If you are running the latest
version of Sriracha, please [report](https://codeberg.org/tslocum/sriracha/issues) the issue.

### Custom pages

Pages and templates may access the database via the following read-only methods:

- [Ban](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Ban)
  - `BanByID(id int) *Ban`
  - `BanByIP(ip string) *Ban`
  - `AllActiveBans() []*Ban`
  - `LiftedBansByIP() []*Ban`
- [Banner](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Banner)
  - `BannerByID(id int) *Banner`
  - `BannerByName(name string) *Banner`
  - `AllBanners() []*Banner`
- [Board](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Board)
  - `BoardByID(id int) *Board`
  - `BoardByDir(dir string) *Board`
  - `UniqueUserPosts(b *Board) int`
  - `AllBoards() []*Board`
- [Category](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Category)
  - `CategoryByID(id int) *Category`
  - `ChildCategories(id int) []*Category`
  - `AllCategories() []*Category`
- [Keyword](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Keyword)
  - `KeywordByID(id int) *Keyword`
  - `KeywordByText(text string) *Keyword`
  - `AllKeywords() []*Keyword`
- [News](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#News)
  - `NewsByID(id int) *News`
  - `AllNews(onlyPublished bool) []*News`
- [Page](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Page)
  - `PageByID(id int) *Page`
  - `PageByPath(path string) *Page`
  - `AllPages() []*Page`
- [Post](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Post)
  - `AllThreads(filter PostFilter, board ...*Board) [][2]int`
  - `AllPostsInThread(filter PostFilter, postID int) []*Post`
  - `AllReplies(filter PostFilter, limit int) []*Post`
  - `PendingPosts() []*Post`
  - `PrunedThreads() []int`
  - `PostByID(postID int) *Post`
  - `PostsByID(postIDs []int) []*Post`
  - `PostsByIP(hash string) []*Post`
  - `PostsByFileHash(hash string, filterBoard *Board) []*Post`
  - `PostByField(b *Board, field string, value any) *Post`
  - `LastPostByIP(board *Board, ip string) *Post`
  - `SearchPosts(filter PostFilter, query string, board ...*Board) []int`
  - `ReplyCount(threadID int) int`
- [Report](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Report)
  - `NumReports(p *Post) int`
  - `PostReported(p *Post, ipHash string) bool`
  - `AllReports() []*Report`
- [Subscription](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Subscription)
  - `SubscriptionByID(id int) *Subscription`
  - `SubscriptionByIP(ip string) *Subscription`
  - `SubscriptionsByEmail(email string) []*Subscription`
  - `SubscriptionsByPost(p *Post, distinct bool, includeBoard bool) []*Subscription`
- [Threshold](https://docs.rocket9labs.com/codeberg.org/tslocum/sriracha/model/#Threshold)
  - `ThresholdByID(id int) *Threshold`
  - `ThresholdTimeout(t *Threshold, ipHash string, now int64) int`
  - `AllThresholds() []*Threshold`

For example, the following custom page will render all moderated posts in the
board with ID #7 by printing their ID, subject and message:

```gohtml
{{$postFilter := 1}}{{/* Only show visible posts. */}}
{{$board := BoardByID 7}}{{/* Fetch board with ID 7 from the database. */}}
{{$threads := AllThreads $postFilter $board}}{{/* Fetch threads in board. */}}
<hr>
Found {{len $threads}} threads.
{{range $i, $thread := $threads}}{{/* Iterate over each thread. */}}
    {{$threadID := index $thread 0}}
    {{$threadReplyCount := index $thread 1}}
    <hr>
    Thread No.{{$threadID}} (Replies: {{$threadReplyCount}})
    {{range $post := AllPostsInThread $postFilter $threadID}}{{/* Iterate over each post in thread. */}}
        <br><br>
        ID: {{$post.ID}}<br>
        Subject: {{$post.Subject}}<br>
        Message {{$post.Message | HTML}}
    {{end}}
{{end}}
<hr>
```

### Browsers

Sriracha web pages follow the [HTML5](https://en.wikipedia.org/wiki/HTML5) specification.
While any web browser should work with Sriracha, only [Firefox](https://firefox.com)
and [Lynx](https://en.wikipedia.org/wiki/Lynx_(web_browser)) are officially supported.

### Custom styles

Use the [style builder](https://sriracha.rocket9labs.com/static/stylebuilder.html)
to quickly prototype new Sriracha styles.

### Memory usage

The following calculation provides the worst-case scenario for memory usage.
This is the expected increase in memory usage when the server is fully saturated
with the maximum number of active connections allowed and every page buffer has
also grown to the maximum size allowed.

```
mem = (maxconns * maxformbuffer) + (numcpu * maxpagebuffer)
```

Replace `numcpu` with the number of CPU cores available to Sriracha.

### Performance

The best way to improve the performance of Sriracha is by adjusting the
configuration of your PostgreSQL database.

See [this page](https://www.postgresql.org/docs/current/runtime-config-resource.html)
for a full list of configuration options related to resource consumption.

On systems with 1GB or less of available memory, the default PostgreSQL
configuration should be used.

On systems with more than 1GB of available memory, the following options should be increased:

#### shared_buffers

Increase this limit from 128MB to 25% of available memory. This option will
impact overall system performance.

#### work_mem

Increase this limit from 4MB to 16MB. This option will impact performance when
there are many rows in the database.

#### maintenance_work_mem

Increase this limit from 64MB to 25% of available memory. This option will
impact performance when PostgreSQL cleans up deleted or outdated table data.

### Caching middleware

Websites with high volumes of new posts may benefit from the use of caching
middleware such as [Pgpool-II](https://pgpool.net/docs/latest/en/html/intro-whatis.html).

However, before adding any caching middleware to your Sriracha server, it is
important to understand how the database and static files are already cached.

Sriracha is optimized to rely on the built-in caching functionality of PostgreSQL,
as well as the built-in caching functionality of the underlying operating system.

Database rows which are accessed often, and well as files which are accessed often,
will be cached in-memory by PostgreSQL and the operating system.

When more than one post is added within a ten second time frame, Sriracha will conserve
system resources by [batching](#post-batching) static page updates together.

Thus, the addition of caching middleware will add unnecessary abstraction and
latency for most websites, and will actually reduce performance.

Always verify any expected performance improvements by running multiple benchmarks
(via the `--benchmark` flag) before and after making any changes.

### Tracing

To print performance metrics to the server console, start Sriracha with the `--trace` flag.
Adding this flag will not significantly affect performance.

Developers can also enable high precision tracing by setting the `SRIRACHA_TRACE` environment variable.
High precision tracing severely degrades system performance and should only be used by developers.
Run `go tool trace file.trace` after shutting down Sriracha to view recorded information.

### Locales

The following locales have partial or full translations:

| Locale    | Description          |
| --        | --                   |
| `en`      | English              |
| `sq`      | Albanian             |
| `zh_Hans` | Chinese (Simplified) |
| `nl`      | Dutch                |
| `fi`      | Finnish              |
| `ru`      | Russian              |

Sriracha relies on the assistance of volunteer translators. If you are multilingual,
please [help translate Sriracha](https://translate.codeberg.org/projects/sriracha/sriracha/).

### Example configuration (config.yml)

```yaml
# Interface language. See locale directory for full list.
locale: "en"

# Directory where board files are written to.
root: "/home/sriracha/public_html"

# Hostname:Port to listen for HTTP connections on.
http: "localhost:8080"

# HTTPS server configuration. Instead of setting these options, you should run
# a frontend such as Caddy with Sriracha, which will manage HTTPS certificates
# automatically and will translate HTTPS requests to HTTP.
#https:     "" # Hostname:Port to listen for HTTPS connections on.
#httpscert: "" # Path to HTTPS certificate file.
#httpskey:  "" # Path to HTTPS private key file.

# Whether the server should reject HTTP/1 connections. When enabled, the server
# will only accept HTTP/2 connections. You should enable this option if possible,
# because HTTP/1 suffers from flaws which can allow attackers to hijack requests.
# If you are using a frontend such as Caddy with Sriracha, you should enable this,
# because the frontend will translate HTTP/1 requests to HTTP/2 when forwarding.
# If you are using Sriracha without a frontend, leave this option disabled.
#rejecthttp1: false

# Client IP address header. Must be set when running behind a frontend.
# When running behind CloudFlare, use CF-Connecting-IP. When running without
# a frontend, leave blank.
#header: "X-Forwarded-For"

# Hash algorithm. Supported algorithms are sha-3 (recommended) and sha-2. Must not change once set.
algorithm: "sha-3"

# Long random string of text used when one-way hashing data. Must not change once set.
saltdata: "CHANGEME_Random_Data_Here_1"

# Long random string of text used when two-way hashing data. Must not change once set.
saltpass: "CHANGEME_Random_Data_Here_2"

# Long random string of text used when generating secure tripcodes. Must not change once set.
salttrip: "CHANGEME_Random_Data_Here_3"

# Long random string of text used when generating identifiers. Must not change once set.
saltident: "CHANGEME_Random_Data_Here_4"

# Hostname:Port to connect to the database.
address: "localhost:5432"

# Database username.
username: "sriracha"

# Database password.
password: "hunter2"

# Database name.
dbname: "sriracha"

# Database connection URL. Allows specifying additional connection options.
# This option supersedes the address, username, password and dbname options.
# You probably don't need this. Configure the database options above instead.
# See https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#ParseConfig
#dburl: "postgresql://sriracha:hunter2@localhost:5432/sriracha"

# Whether identifier hashes are enabled. Identifier hashes are generated based
# on IP hashes. When enabled, staff may view and delete all posts created by an
# IP address, and boards may optionally display identifier hashes to visitors.
#identifiers: false

# Available stylesheets. The first listed stylesheet will be the default.
#styles:
#  - "futaba"
#  - "burichan"
#  - "sriracha"

# Custom template directory. Leave blank to use official templates. Template
# files in this directory will override official templates of the same name.
#template: "/home/sriracha/template"

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
#mailfrom:     ""          # Notification "From" email address. Required.
#mailreplyto:  ""          # Notification "Reply-To" email address.
#maildomains:  ""          # Regular expression specifying allowed email address domains.

# Notification batch durations. These options only apply when a mail server is configured.
#mentions:      60   # Duration (in minutes) mention notifications are batched together.
#notifications: 1440 # Duration (in minutes) all other notifications are batched together.

# Require two-factor authentication. When enabled, staff must enter a TOTP passcode in
# addition to their username and password. Enabling this option increases security.
#require2fa: false

# Do not record post author IP addresses. When this option is enabled, it is
# not possible for staff members to ban visitors. You should only enable this
# option if you are running Sriracha somewhere other than the public Internet.
#noip: false

# Minimum static page buffer size. This is the initial size of each buffer.
# You probably don't need to change this. If you do change it, confirm any
# expected performance improvements by running benchmarks before and after.
#minpagebuffer: 500000 # 500 KB.

# Maximum static page buffer size. When exceeded, the buffer size is reset.
# You probably don't need to change this. If you do change it, confirm any
# expected performance improvements by running benchmarks before and after.
# Memory usage will increase by numcpu * maxpagebuffer when fully saturated.
#maxpagebuffer: 4000000 # 4 MB.

# Maximum form buffer size. Uploaded files are first read into the form buffer.
# When the total upload exceeds this limit, remaining data is written to disk.
# Each active connection has its own form buffer. You should only change this
# if you want to handle requests larger than 16 MB without buffering to disk.
#maxformbuffer: 16000000 # 16 MB.

# Maximum active connections. Connections exceeding this limit are still accepted
# but are not read from until an active connection slot becomes available.
# Memory usage will increase by maxconns * maxformbuffer when fully saturated.
#maxconns: 16

# Access required to perform an action. Default values for all actions are listed below.
#
# Format: mod / admin / super-admin / disable (disallow all roles)
#access:
#  ban.add:         "mod"
#  ban.shorten:     "admin"
#  ban.lengthen:    "mod"
#  ban.lift:        "admin"
#  banfile.add:     "mod"
#  banfile.lift:    "admin"
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
#  post.lock:       "mod"
#  post.sticky:     "mod"
#  post.spoiler:    "mod"
#  post.move:       "mod"
#  post.delete:     "mod"

# Audit database connection URL. When configured, log entries are duplicated
# to a secondary database. The audit database role must be forbidden from
# executing all commands except SELECT and INSERT to prevent log tampering.
# See https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#ParseConfig
#audit: "postgresql://sriracha_audit:hunter2@localhost:5432/sriracha_audit"

# Free disk space warning threshold. A warning message is displayed to staff
# members if and when the available free space drops below this threshold.
#warnfree: 2000000000 # 2 GB.

# Minimum free disk space. Many features, including adding new posts, will be
# disabled if and when the available free space drops below this threshold.
#minfree: 500000000 # 500 MB.

# Supported upload file types. Specify a file extension and a MIME type to
# enable uploading files of that type. You may specify an image to use as the
# thumbnail for all uploads of that type, or 'none' to not create a thumbnail.
# Otherwise, thumbnails are generated automatically based on the uploaded file.
# To generate thumbnails for videos, ffmpeg must be installed. SVG images allow
# embedded JavaScript, which is dangerous. Do not accept untrusted SVG images.
# Note: Opus audio files are detected as audio/ogg.
#
# Format: ext mime thumbnail
uploads:
  - "apng  image/apng"
  - "apng  image/vnd.mozilla.apng"
  - "avif  image/avif"
  - "bmp   image/bmp"
  - "gif   image/gif"
  - "jpg   image/jpeg"
  - "jpg   image/pjpeg"
  - "png   image/png"
  - "tiff  image/tiff"
  - "aac   audio/aac"
  - "flac  audio/flac"
  - "m4a   audio/x-m4a"
  - "midi  audio/midi"
  - "mp3   audio/mp3"
  - "mp3   audio/mpeg"
  - "mp4   audio/mp4"
  - "ogg   audio/ogg"
  - "wav   audio/wav"
  - "wav   audio/wave"
  - "wav   audio/x-wav"
  - "weba  audio/webm"
  - "avi   video/x-msvideo"
  - "mkv   video/x-matroska"
  - "mp4   video/mp4"
  - "mpeg  video/mpeg"
  - "ogv   video/ogg"
  - "webm  video/webm"
  - "swf   application/x-shockwave-flash  swf.png"
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
  @staticFiles {
    path /banner/*
    path /captcha/*
    path /static/*
    path_regexp ^.*/(src|thumb)/.*$
  }
  header @staticFiles Cache-Control "public, max-age=1209600, immutable"

  # Forward /sriracha requests to Sriracha.
  reverse_proxy /sriracha* h2c://localhost:8080

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
| [BBCode](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/bbcode/bbcode/bbcode.go) | Format BBCode in post messages. |
| [Fortune](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/fortune/fortune/fortune.go) | Give your visitors some good luck (or bad). |
| [IRC](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/irc/irc/irc.go) | Send server event notifications. |
| [Password](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/password/password/password.go) | Require specific passwords to post. |
| [Robot9000](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/robot9000/robot9000/robot9000.go) | Require post messages to be unique. |
| [Statistics](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/statistics/statistics/statistics.go) | View statistics for each board. |
| [Wordfilter](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/wordfilter/wordfilter/wordfilter.go) | Find and replace text in post messages. |

### Using plugins

#### Native

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

#### Docker

To use custom Sriracha plugins inside a container using Docker, you will need to build
and use your own Sriracha container image.

Download the the latest official Sriracha release source code by clicking
'Source code' on the [Releases](https://codeberg.org/tslocum/sriracha/releases) page.

After extracting the source code, open a terminal and change directories to
the extracted Sriracha source code and run the following command:

```
mkdir customplugin
```

Then create a sub-directory within `customplugin` for each custom pugin.

To build a Sriracha container image including all custom plugins, run the
following command after replacing 1.0.0 with the downloaded version:

```
sudo docker build -t sriracha --build-arg version=1.0.0 .
```

Once complete, instead of starting a container using `tslocum/sriracha`
(downloaded from Docker Hub), use the locally built image `sriracha`.

Sriracha will not load any custom plugins by default. You will need to provide
a custom plugin file or directory path when starting the server.

For example, to load all custom plugins included in an image, provide the
following directory when starting Sriracha:

```
sriracha /usr/share/sriracha/customplugin
```

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

A plugin's `About` method is only called once.

Plugins may subscribe to receive and handle [events](#plugin-events) by implementing additional interfaces.

### Plugin configuration

Plugins may optionally specify any number of configuration options:

```go
// PluginWithConfig describes the required methods for a plugin with configuration options.
type PluginWithConfig interface {
  Config() []PluginConfig
}
```

A plugin's `Config` method is only called once.

The following configuration option types are available:

- Boolean
- Integer
- Float
- Enum
- String
- Board

Boolean options may only have one value. Options of any other type may have one or multiple values.

Plugin configuration options may be viewed and modified in the 'Plugins' page of the management panel.

An example how to implement a plugin with configuration options is available in
the [Fortune](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/fortune/fortune/fortune.go) plugin.

### Plugin events

Plugins may subscribe to receive one or more types of events by implementing
the associated event handlers.

For example, a plugin which modifies new posts would implement [PluginWithPost](https://pkg.go.dev/codeberg.org/tslocum/sriracha#PluginWithPost)
to subscribe to Post events.

After Post events are handled, [Create](https://pkg.go.dev/codeberg.org/tslocum/sriracha#PluginWithCreate) events are sent,
which plugins may use to cancel a new post before it is inserted into the database.

After Create events are handled, the post is inserted into the database and [Insert](https://pkg.go.dev/codeberg.org/tslocum/sriracha#PluginWithInsert)
events are sent, which plugins may use to process finalized posts.

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

#### Cron event

Cron events are sent once during startup for each plugin. Plugins may return
a positive integer to specify a delay (in seconds) until the next Cron event.
Plugins may return a negative integer or zero instead to delay until midnight.

Plugins should use Cron events to perform any polling or processing of Sriracha
data, because the server does not handle web requests during the Cron event.

Processing of external data should be handled in the background instead. Plugins
with excessively long Cron events will cause pending web requests to fail.

```go
// PluginWithCron describes the required methods for a plugin subscribing to cron events.
type PluginWithCron interface {
	Cron(db DB) (int, error)
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

Sriracha supports importing posts from:

- [Sriracha](#import-posts)
- [TinyIB](#import-posts-from-tinyib)
- [vichan](#import-posts-from-vichan)

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

### Import posts from vichan

**Note:** Sriracha does not have the same features as vichan.

To import posts from a vichan database, provide the relevant connection details via `SRIRACHA_IMPORT` when starting Sriracha:

```
SRIRACHA_IMPORT='mysql://user:pass@tcp(localhost)/dbname' sriracha
```

Then follow the import instructions above from "4. Visit the management panel" onward.

Embed services (such as YouTube) must be added in Sriracha and enabled in the
settings of each Sriracha board before importing posts with embeds.

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

The following sentinel values are used:

- `fileoriginal` has prefix `!` - Spoiler thumbnail
- `fileoriginal` = `?a` - Attachment deleted by author
- `fileoriginal` = `?s` - Attachment deleted by staff

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
refresh after five minutes have passed. When permission to send notifications has
been granted, a notification will be displayed when a moderation request is added.

#### Banning IP addresses

Single IP addresses and IP address ranges may be banned. To ban an IP address
range, use a wildcard (*) at the end of the range prefix:

`192.168.1.*`

#### Browsing in mod mode

Mod mode is a tool staff members may use to moderate one or more posts.
When browsing in mod mode, the following moderation links are displayed:

**# ? Ø V M S L D B D&B**

- **#** Archive or unarchive thread
- **?** Spoiler or unspoiler thumbnail (when a thumbnail is present and spoilers are enabled)
- **Ø** Delete attachment (when an attachment is present and the post has a message)
- **V** View posts by author (when identifiers are enabed)
- **M** Move thread
- **S** Sticky or unsticky thread
- **L** Lock or unlock thread
- **D** Delete post
- **B** Ban post author
- **D&B** Delete post and ban post author

A shortcut for accessing mod mode is available when logged in. View any index
or thread page normally, scroll to the bottom of the page and click the delete
button. If you are logged in to a staff account, you will be redirected to the
page you were just viewing with mod mode enabled.

#### Bulk moderation

To moderate multiple posts, select the checkboxes of any relevant posts and scroll
to the bottom of the page. When browsing normally, click the 'Delete' button without
entering a password. When browsing in mod mode, click the 'Moderate' button.

Any existing bans for shorter durations will be replaced.

#### Archive

When the configured maximum thread limit is exceeded, the oldest threads are pruned.

The Archive setting of each board determines how pruned threads are handled:

- **Disable** - Pruned threads are deleted automatically.
- **Manual** - Pruned threads appear in the management panel, allowing staff to
choose which threads are deleted or archived. This is the default.
- **Automatic** - Pruned threads are archived automatically.

#### News

When enabled, all staff have access to post news entries. Staff may also update
existing news entries which have been shared by other staff members.

Leave a news entry's Date field blank to draft a news entry only visible to staff.

Enter a date and time in the future to hide the news entry from visitors until then.

Enter the present date and time or a past date and time to show a news entry to visitors.

When the Site Index is enabled, the latest news entry will be displayed on the homepage.

### Administrator guide

As an administrator, in addition to moderator privileges you may:

- Lift bans
- Add boards
- Update boards
- Add keywords
- Update keywords
- Delete keywords
- Delete news
- Update settings

#### Duplicate attachments

The 'Instance Limit' option of each board specifies how many instances of an
attachment (file or embed) are allowed, as well as where to search for posts.

When the 'Instance Limit' is set to a positive number, all boards are searched for
existing instances. When set to a negative number, only the board which is being
posted to is searched. When set to zero, duplicate attachment checks are disabled.

By default, only one instance of an attachment is allowed across the entire site.
When someone uploads a file or embeds a URL which has already been posted, they
will receive an error message containing a link the existing post.

#### Keywords

Keywords are [regular expressions](https://en.wikipedia.org/wiki/Regular_expression) which,
when detected, trigger automatic moderation.

A built-in keyword tester is included. To test building a regular expression,
use an expression testing website with the Go (or "Golang") format selected.

Sriracha normalizes text before detecting keywords. This avoids the need to
create multiple keywords for each possible accent character variation.

Write keywords using only characters without accents.

#### Global settings

Some board, banner and keyword settings may be configured globally. Administrators
may specify whether settings are local or global via the Settings page.

When a new board, banner or keyword is added, global settings are copied from existing entries.

When a global setting is modified, it is applied to all existing boards, banners or keywords.

A globe icon is displayed when configuring a global setting.

To apply a newly specified global setting, visit the 'Update' page of any
relevant board, banner or keyword, scroll down and click the 'Update' button.

#### Thresholds

Thresholds are automatic moderation rules which take effect when visitors exceed
a specific amount of new posts or reports within a specific duration.

Thresholds may only apply to individual visitors, or they may apply to all visitors.
Likewise, they may only apply to individual boards, or the entire site.

Only new posts and reports which exceed a threshold are effected. Existing posts
and reports are preserved.

To configure post and report thresholds, click 'Manage Thresholds' at the bottom
of the Settings page.

The following thresholds are configured by default:

- When an individual visitor adds more than 1 post anywhere within 30s, delete.
- When an individual visitor adds more than 1 thread anywhere within 5m, delete.
- When an individual visitor adds more than 10 reports anywhere within 1h, delete.

#### Date/time format

By default, Sriracha formats timestamps as `2006/01/02<wbr>(Mon)<wbr>15:04:05`.
Administrators may customize the date/time format in the Settings page.

See the [Go time package documentation](https://pkg.go.dev/time#pkg-constants)
for information on how to write alternative formats.

For example, a 12-hour alternative of the default format would be `2006/01/02<wbr>(Mon)<wbr>03:04:05PM`.

### Super-administrator guide

As a super-administrator, you have unrestricted access.

#### Rebuild static pages

Super-administrators may manually rebuild static pages. Visit the Settings page,
scroll to the end and click the Update button to rebuild all static pages.

During normal operation, manually rebuilding static pages is unnecessary.
Whenever Sriracha data is modified, static pages are automatically rebuilt.

If you find yourself needing to manually rebuild static pages often, and you
are not running Sriracha in development mode, please [report](https://codeberg.org/tslocum/sriracha/issues)
the issue.

#### Rebuild name blocks

Each post has a `nameblock` field containing the poster's name, tripcode and
identifier, as well as the date/time the post was created. Super-administrators
may rebuild all `nameblock` fields by visiting `/sriracha/?rebuildNameblocks`.

#### Rebuild reflinks

Super-administrators may rebuild all reflinks `>>###` by visiting `/sriracha/?rebuildReflinks`.

#### Verify memory configuration

Super-administrators may view detailed information related to memory usage,
including the potential increase in memory usage when the server is fully saturated,
by visiting `/sriracha/?memoryConfig`.

#### Verify IP address resolution

To support banning visitors, Sriracha must be able to resolve remote IP addresses,
which is only possible when the header server option is configured correctly.
Super-administrators may verify remote IP address resolution by visiting `/sriracha/?remoteAddress`.

#### Scan for unexpected files

While Sriracha always attempts to clean up any files related to posts, it is
possible for files which are not attached to any posts to accumulate in board
directories. Super-administrators may scan for any unexpected files by visiting `/sriracha/?scanFiles`.
