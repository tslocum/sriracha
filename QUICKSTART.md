# Sriracha Quick Start
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

## Install Docker

Install [Docker](https://docker.com) and [Docker Compose](https://docs.docker.com/compose/install).

## Download and extract Sriracha source code

Click 'Source code' on the latest [release page](https://codeberg.org/tslocum/sriracha/releases/latest).

## Create directories and update volume paths

Create four directories for the following data:

- Sriracha configuration
- Sriracha static files
- PostgreSQL data
- Caddy configuration

By default, the included `docker-compose.yml` file will mount these directories from `/home/sriracha`.

Update the volume source paths in the file `docker-compose.yml` to point to the directories you created instead.

For example, if you created four directories named `srirachaconf`, `srirachapublic`, `pgdata` and `caddyconf`:

```yaml
services:
  sriracha:
    # (Snip)
    volumes:
      - /home/myuser/srirachaconf:/etc/sriracha # Configuration drectory containing config.yml
      - /home/myuser/srirachapublic:/public # Root directory containing board files.
  sriracha_db:
    # (Snip)
    volumes:
      - /home/myuser/pgdata:/var/lib/postgresql/data # Database directory containing PostgreSQL data.
  httpd:
    # (Snip)
    volumes:
      - /home/myuser/caddyconf:/config/caddy # Caddy configuration directory containing Caddyfile.
      - /home/myuser/srirachapublic:/mnt/sriracha # Root directory containing board files.
```

## Configure Sriracha

Create a file in `srirachaconf` named `config.yml` and paste the [example configuration](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#example-configuration-config-yml). A quick-copy button appears when the example text is hovered.

Update the Sriracha configuration file options as desired. Note that any paths
must be from the perspective of the Docker container.

For example, you should set the `root` option to `/public`, not `/home/myuser/srirachapublic`.

Set the `address` option to `sriracha_db`, the name of the Sriracha database service.

If you are using Caddy with Sriracha, set the `header` option to `X-Forwarded-For`.
If you are not using Caddy, leave the `header` option blank.

See the [Configure](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#configure)
section of the Sriracha Manual for more information.

Change the owner of the `srirachaconf` and `srirachapublic` directories:

```
chown -R 1000:1000 srirachaconf srirachapublic
```

## Configure PostgreSQL

Update the `sriracha_db` environment variables in `docker-compose.yml`.

Edit the database options in the Sriracha `config.yml` file to match the newly
updated environment variables.

Update the `command` parameter of `sriracha_db` to specify PostgreSQL resource usage limits.

The example values included in `docker-compose.yml` are for a system with 2 GB
of available memory. See the [Performance](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#performance) section for optimal values.

Change the owner of the `pgdata` directory:

```
chown -R 70:70 pgdata
```

## Configure Caddy

**Note:** If you only want to test Sriracha locally, skip ahead to the [next section](#disable-caddy).

Create a file in `caddyconf` named `Caddyfile` and paste the [example configuration](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#example-reverse-proxy-using-caddy-caddyfile).

Update the Caddy configuration file options as desired. Note that any paths
must be from the perspective of the Docker container.

Change the owner of the `caddyconf` directory:

```
chown -R 1000:1000 caddyconf
```

## Disable Caddy

**Note:** If you want to expose your Sriracha server publicly to other people, leave Caddy enabled.

If you are only interested in testing Sriracha locally, you should access Sriracha directly without Caddy.

Comment out or delete any lines in `docker-compose.yml` defining or referencing the `httpd` service.

Afterward, you should only have two Docker Compose services: `sriracha` and `sriracha_db`.

Update the `sriracha` service to expose port 8080 locally:

```yaml
services:
  sriracha:
    # (Snip)
    ports:
      - "127.0.0.1:8080:8080"
```

## Start Sriracha

Run the following command to start the services defined in `docker-compose.yml`:

```
docker compose up -d
```

View the log output of a service so far:

```
docker compose logs sriracha
```

Monitor the ongoing log output of a service:

```
docker compose logs -f sriracha
```

Restart services:

```
docker compose restart
```

Stop services:

```
docker compose stop
```

Stop and remove associated containers:

```
docker compose down
```

## Log in

The management panel is accessible at `/sriracha`. The default
super-administrator account has a username and password of `admin`.

### With Caddy

If you are using Caddy, access the management panel at the domain you specified
in the `Caddyfile`.

For example, if your domain was `zoopz.org`, you could access the management
panel at `https://zoopz.org/sriracha`.

### Without Caddy

If you are not using Caddy, access the management panel at `http://127.0.0.1:8080/sriracha`.

## Read the Sriracha Manual

See the [Sriracha Manual](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#configure)
for next steps.
