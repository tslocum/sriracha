# Sriracha - Imageboard and forum
[![GoDoc](https://codeberg.org/tslocum/godoc-static/raw/branch/main/badge.svg)](https://pkg.go.dev/codeberg.org/tslocum/sriracha#section-documentation)
[![Translate](https://translate.codeberg.org/widget/sriracha/sriracha/svg-badge.svg)](https://translate.codeberg.org/projects/sriracha/sriracha/)
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

This application is in pre-alpha development. Here be dragons.

Only Linux, FreeBSD and macOS are supported.

## Feature Parity with TinyIB

- [X] GIF, JPG, PNG, SWF, MP4 and WebM upload
- [X] YouTube, Vimeo and SoundCloud embedding
- [X] CAPTCHA
- [X] Reference links `>>###`
- [X] Delete posts via password
- [X] Report posts
- [X] Thread catalog
- [ ] Fetch new replies automatically
- [X] Support additional languages
- [X] Management panel:
  - [X] Automatically moderate new posts using [regular expressions](https://en.wikipedia.org/wiki/Regular_expression)
  - [X] Ban offensive/abusive posters across all boards
  - [X] Post using admin or mod capcode
  - [X] Post using raw HTML
  - [X] Account system:
    - [X] Super administrators (all privileges)
    - [X] Administrators (all privileges except managing accounts and deleting boards)
    - [X] Moderators (may only add bans, approve/delete posts and sticky/lock threads)

## Documentation

### Server

See [CONFIGURATION.md](https://codeberg.org/tslocum/sriracha/src/branch/main/CONFIGURATION.md)
for info on how to configure Sriracha.

### Plugins

See [PLUGINS.md](https://codeberg.org/tslocum/sriracha/src/branch/main/PLUGINS.md)
for info on how to build and use Sriracha plugins.

### Migrate

See [MIGRATE.md](https://codeberg.org/tslocum/sriracha/src/branch/main/MIGRATE.md)
for info on how to migrate from [TinyIB](https://codeberg.org/tslocum/tinyib).

## Translate

Translation is handled [online](https://translate.codeberg.org/projects/sriracha/sriracha/).

## License

This application is licensed under [LGPLv3](https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE).
If you modify the source code of this application, you must share the full
source code of your changes publicly for free. You may, however, link with this
application using proprietary shared libraries, so long as the base application
(Sriracha) remains unmodified. If your only changes are to create proprietary
shared libraries, and these librarires would work with other installations of
Sriracha because you did not make any modifications to Sriracha's source code,
then you do not need to release the source code of your shared libraries.
