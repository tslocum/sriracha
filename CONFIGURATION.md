# Sriracha Configuration
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

When starting Sriracha, the path to the server configuration file may be
specified via the `--config` option:

`sriracha --config /path/to/config.yml`

If no configuration file path is specified, the default path
`~/.config/sriracha/config.yml` is used.

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

## Example configuration (config.yml)

```yaml
# Interface language. See locale directory for available languages.
locale: en

# Directory where board files are written to.
root: /home/sriracha/public_html

# Address:Port to listen for HTTP connections on.
serve: localhost:8080

# Client IP address header. Must be set when running behind a reverse proxy.
# When running behind CloudFlare, use CF-Connecting-IP. When running without
# a proxy, leave blank.
header: X-Forwarded-For

# Long random string of text used when one-way hashing data. Must not change once set.
saltdata: CHANGEME_Random_Data_Here_1

# Long random string of text used when two-way hashing data. Must not change once set.
saltpass: CHANGEME_Random_Data_Here_2

# Long random string of text used when generating secure tripcodes. Must not change once set.
salttrip: CHANGEME_Random_Data_Here_3

# Address:Port to connect to the database.
address: localhost

# Database username.
username: sriracha

# Database password.
password: hunter2

# Database name.
dbname: sriracha

# Custom template directory. Leave blank to use standard templates. Template
# files in this directory will override standard templates of the same name.
template: /home/sriracha/template

# Supported upload file types. Specify a MIME type and a file extension to
# enable uploading files of that type. You may specify an image to use as the
# thumbnail for all uploads of that type, or 'none' to not create a thumbnail.
# Otherwise, thumbnails are generated automatically based on the uploaded file.
# To generate video thumbnails, ffmpeg must be installed.
#
# Format: ext mime thumbnail
uploads:
  - jpg image/jpeg
  - jpg image/pjpeg
  - png image/png
  - gif image/gif
  - wav audio/wav
  - wav audio/wave
  - wav audio/x-wav
  - aac audio/aac
  - ogg audio/ogg
  - flac audio/flac
  - opus audio/opus
  - mp3 audio/mp3
  - mp3 audio/mpeg
  - mp4 audio/mp4
  - mp4 video/mp4
  - webm audio/webm
  - webm video/webm
  - swf application/x-shockwave-flash swf.png
```

## Example reverse proxy using caddy (Caddyfile)

```
zoopz.org, www.zoopz.org {
  reverse_proxy http://localhost:8080
}
```
