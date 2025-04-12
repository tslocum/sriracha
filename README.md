# sriracha - Imageboard and forum
[![GoDoc](https://codeberg.org/tslocum/godoc-static/raw/branch/main/badge.svg)](https://pkg.go.dev/codeberg.org/tslocum/sriracha#section-documentation)
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

This application is in pre-alpha development. Here be dragons.

Only Linux, FreeBSD and macOS are supported.

sriracha will eventually serve as a replacement for imageboards running [TinyIB](https://codeberg.org/tslocum/tinyib).

## Plugins

sriracha supports building and loading plugins via shared library files. Plugins
are not sandboxed in any way. Every plugin has full access to the system. For
this reason, you should only load plugins you personally built after inspecting
the source code. Never load a compiled plugin built by someone else.

Official plugins are located in the [plugin](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin)
directory.

### Instructions

To build a plugin, run the following commands:

```
cd /path/to/sriracha/plugin/fortune
go build -buildmode=plugin
```

This will compile the fortune plugin as `fortune.so`.

To load a plugin, run the following command:

```
sriracha --config=/path/to/config.yml /path/to/fortune.so
```

Multiple plugin paths may be provided. When a directory is provided, all plugins
located in the directory are loaded.

### Compatibility

Only plugins built using the same version of sriracha may be used.

If you attempt to load a plugin and see an error such as:

```
failed to load plugin ./plugin/fortune/fortune.so: plugin.Open("./plugin/fortune/fortune"): plugin was built with a different version of package codeberg.org/tslocum/sriracha
```

The solution is to rebuild all plugins and sriracha itself.

## Documentation

Documentation is available via [godoc](https://pkg.go.dev/codeberg.org/tslocum/sriracha#section-documentation).

## License

This application is licensed under [LGPLv3](https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE).
If you modify the source code of this application, you must share the full
source code of your changes publicly for free. You may, however, link with this
application with proprietary shared libraries, so long as the base application
(sriracha) remains unmodified. If your only changes are to create proprietary
shared libraries, and these librarires would work with other installations of
sriracha because you did not make any modifications to sriracha's source code,
then you do not need to release the source code of your shared libraries.
