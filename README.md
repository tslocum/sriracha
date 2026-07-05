# Sriracha - Imageboard and forum server
[![GoDoc](https://codeberg.org/tslocum/godoc-static/raw/branch/main/badge.svg)](https://pkg.go.dev/codeberg.org/tslocum/sriracha#section-documentation)
[![Translate](https://translate.codeberg.org/widget/sriracha/sriracha/svg-badge.svg)](https://translate.codeberg.org/projects/sriracha/sriracha/)
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

[**Browse the read-only demo**](https://sriracha.rocket9labs.com/img/)

- Host imageboards and forums online or offline
- Organize files via boards and threads
- Organize notes via news entries

## Features

- Upload files matching MIME type whitelist
- Embed external media (YouTube, Vimeo and SoundCloud)
- Reference links `>>###`
- CAPTCHA
- Report posts
- Overboard
- Thread catalog
- Oekaki (drawings)
- Fetch new replies automatically
- Extend with [plugins](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#plugins)
- Send [email notifications](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#email-notifications)
- Customize [HTML templates](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#custom-templates)
- Translate into [additional languages](https://translate.codeberg.org/projects/sriracha/sriracha/)
- Play SWF files with the included [ruffle emulator](https://ruffle.rs)
- View Shift JIS text art with the included [submona web font](https://github.com/pera/submona-web-font)
- Management panel
  - Configure which roles have access to management and moderation actions
  - Automatically moderate new posts using regular expressions
  - Ban offensive/abusive visitors across all boards
  - Post using admin or mod capcode
  - Post using raw HTML

Most features do not require JavaScript and will degrade gracefully when necessary.

Sriracha requires very little processing power, and will typically use less than 50 MB of memory.

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

Unofficial support is also available via [/help/](https://sriracha.rocket9labs.com/help/) and [Matrix](https://matrix.to/#/#sriracha:matrix.org).

## License

This application is licensed under [LGPLv3](https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE).
If you modify the source code of this application, you must share the full
source code of your changes publicly for free. You may, however, link with this
application using proprietary shared libraries, so long as the base application
(Sriracha) remains unmodified. If your only changes are to create proprietary
shared libraries, and these librarires would work with other installations of
Sriracha because you did not make any modifications to Sriracha's source code,
then you do not need to release the source code of your shared libraries.
