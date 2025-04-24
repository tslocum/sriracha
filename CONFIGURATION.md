# Sriracha Configuration
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

When starting Sriracha, the path to the server configuration file may be
specified via the `--config` option:

`sriracha --config /path/to/config.yml`

If no configuration file path is specified, the default path
`~/.config/sriracha/config.yml` is used.

[PostgreSQL](https://www.postgresql.org) is the only supported database system.

Only HTTP requests are served by Sriracha. To serve HTTPS requests you must run
Sriracha behind a web server, such as [caddy](https://caddyserver.com), which
forwards the HTTPS requests to Sriracha as plain HTTP. When running behind a web
server, the header server option must be set appropriately. Most web servers use
`X-Forwarded-For` to specify the client IP address.

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
saltdata: CHANGEME_Random_Data_Here

# Long random string of text used when two-way hashing data. Must not change once set.
saltpass: CHANGEME_More_Random_Data

# Minimum number of database connections to maintain in the pool.
min: 1

# Maximum number of database connections to maintain in the pool.
max: 4

# Address:Port to connect to the database.
address: localhost

# Database username.
username: sriracha

# Database password.
password: hunter2

# Database name.
dbname: sriracha
```

## Example reverse proxy using caddy (Caddyfile)

```yaml
zoopz.org, www.zoopz.org {
  reverse_proxy http://localhost:8080
}
```
