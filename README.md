# Sriracha - Imageboard and forum server
[![GoDoc](https://codeberg.org/tslocum/godoc-static/raw/branch/main/badge.svg)](https://pkg.go.dev/codeberg.org/tslocum/sriracha#section-documentation)
[![Translate](https://translate.codeberg.org/widget/sriracha/sriracha/svg-badge.svg)](https://translate.codeberg.org/projects/sriracha/sriracha/)
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

[**Browse the read-only demo**](https://sriracha.rocket9labs.com/img/)

Sriracha requires very little processing power and will typically use less than 250 MB of memory.

Most features do not require JavaScript and will degrade gracefully when necessary.

## Features

- Upload files matching [MIME type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/MIME_types/Common_types) whitelist
- [Embed](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#embed-services) external media (e.g. YouTube, Vimeo, SoundCloud)
- Utilize all CPU cores to build static HTML pages
- Search for posts by subject and message
- Preview reference links when hovered
- Fetch new posts automatically
- Reference links `>>###`
- Report posts
- CAPTCHA
- Overboard
- Thread catalog
- Oekaki (drawings)
- Extend with [plugins](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#plugins)
- Send [email notifications](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#email-notifications)
- Customize [HTML templates](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#custom-templates)
- Play SWF files with the included [ruffle emulator](https://ruffle.rs)
- View Shift JIS text art with the included [submona web font](https://github.com/pera/submona-web-font)
- Translate the entire site or individual boards into [any language](https://translate.codeberg.org/projects/sriracha/sriracha/)
- [Deploy within minutes](https://codeberg.org/tslocum/sriracha/src/branch/main/QUICKSTART.md) using [Docker](https://hub.docker.com/r/tslocum/sriracha/tags) and [Docker Compose](https://codeberg.org/tslocum/sriracha/src/branch/main/docker-compose.yml)
- Management panel
  - Configure which roles have access to management and moderation actions
  - Ban offensive/abusive visitors across all boards
  - Post using raw HTML
  - Post using admin or mod capcode
  - Hide new posts until [approved](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#approving-posts)
  - Delete and/or ban [multiple posts](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#bulk-moderation)
  - Create custom HTML [pages](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#custom-pages)
  - Automatically moderate new posts using regular expressions and [thresholds](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#thresholds)
  - Specify which settings are configured individually or [globally](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#global-settings)
  - Secure your account with [two-factor authentication](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#two-factor-authentication)

Import posts from [TinyIB](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#import-posts-from-tinyib)
or [vichan](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#import-posts-from-vichan).
Vote on which software to support next [here](https://codeberg.org/tslocum/sriracha/issues/122).

[Tofu](https://codeberg.org/tslocum/tofu) is the official thread watcher for Sriracha imageboards and forums.

## Documentation

See [MANUAL.md](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md)
for info on how to install and use Sriracha.

A [quick start](https://codeberg.org/tslocum/sriracha/src/branch/main/QUICKSTART.md)
installation guide is also available.

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
shared libraries, and these libraries would work with other installations of
Sriracha because you did not make any modifications to Sriracha's source code,
then you do not need to release the source code of your shared libraries.
