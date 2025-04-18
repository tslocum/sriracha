# Sriracha - Imageboard and forum
[![GoDoc](https://codeberg.org/tslocum/godoc-static/raw/branch/main/badge.svg)](https://pkg.go.dev/codeberg.org/tslocum/sriracha#section-documentation)
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

This application is in pre-alpha development. Here be dragons.

Only Linux, FreeBSD and macOS are supported.

Sriracha will eventually serve as a replacement for imageboards running [TinyIB](https://codeberg.org/tslocum/tinyib).

## Feature Parity with TinyIB

- [ ] GIF, JPG, PNG, SWF, MP4 and WebM upload.
- [ ] YouTube, Vimeo and SoundCloud embedding.
- [ ] CAPTCHA.
- [ ] Reference links. `>>###`
- [ ] Fetch new replies automatically.
- [ ] Delete posts via password.
- [ ] Report posts.
- [X] Block keywords.
- [X] Management panel:
  - [ ] Post using raw HTML.
  - [X] Account system:
    - [X] Super administrators (all privileges)
    - [ ] Administrators (all privileges except account management)
    - [ ] Moderators (only able to sticky threads, lock threads, approve posts and delete posts)
    - [X] Ban offensive/abusive posters across all boards.

## Documentation

### Server

See [CONFIGURATION.md](https://codeberg.org/tslocum/sriracha/src/branch/main/CONFIGURATION.md)
for info on how to configure Sriracha.

### Plugins

See [PLUGINS.md](https://codeberg.org/tslocum/sriracha/src/branch/main/PLUGINS.md)
for info on how to build and use Sriracha plugins.

## License

This application is licensed under [LGPLv3](https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE).
If you modify the source code of this application, you must share the full
source code of your changes publicly for free. You may, however, link with this
application using proprietary shared libraries, so long as the base application
(Sriracha) remains unmodified. If your only changes are to create proprietary
shared libraries, and these librarires would work with other installations of
Sriracha because you did not make any modifications to Sriracha's source code,
then you do not need to release the source code of your shared libraries.
