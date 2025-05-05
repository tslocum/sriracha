# Sriracha - Imageboard and forum
[![GoDoc](https://codeberg.org/tslocum/godoc-static/raw/branch/main/badge.svg)](https://pkg.go.dev/codeberg.org/tslocum/sriracha#section-documentation)
[![Translate](https://translate.codeberg.org/widget/sriracha/sriracha/svg-badge.svg)](https://translate.codeberg.org/projects/sriracha/sriracha/)
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

A [**read-only demo**](https://sriracha.rocket9labs.com/img/) is available.

## Features

- GIF, JPG, PNG, SWF, MP4 and WebM upload
- YouTube, Vimeo and SoundCloud embedding
- Reference links `>>###`
- Delete posts via password
- CAPTCHA
- Report posts
- Thread catalog
- Fetch new replies automatically
- Translate into additional languages
- Management panel:
  - Automatically moderate new posts using [regular expressions](https://en.wikipedia.org/wiki/Regular_expression)
  - Ban offensive/abusive posters across all boards
  - Post using admin or mod capcode
  - Post using raw HTML
  - Account system:
    - Super-administrators (all privileges)
    - Administrators (all privileges except managing accounts and deleting boards)
    - Moderators (may only add bans, approve/delete posts and sticky/lock threads)

## Documentation

See [MANUAL.md](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md)
for info on how to install and use Sriracha.

## Translate

Translation is handled [online](https://translate.codeberg.org/projects/sriracha/sriracha/).

## Support

**Note:** Support is only available for official Sriracha releases running without any custom templates.

  1. Ensure you are running the latest version of Sriracha.
  2. Review the [open issues](https://codeberg.org/tslocum/sriracha/issues).
  3. Open a [new issue](https://codeberg.org/tslocum/sriracha/issues/new).

## License

This application is licensed under [LGPLv3](https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE).
If you modify the source code of this application, you must share the full
source code of your changes publicly for free. You may, however, link with this
application using proprietary shared libraries, so long as the base application
(Sriracha) remains unmodified. If your only changes are to create proprietary
shared libraries, and these librarires would work with other installations of
Sriracha because you did not make any modifications to Sriracha's source code,
then you do not need to release the source code of your shared libraries.
