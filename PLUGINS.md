# Sriracha Plugins
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

Sriracha supports building and loading plugins via shared library files. Plugins
are not sandboxed in any way. Every plugin has full access to the system. For
this reason, you should only load plugins you personally built after inspecting
the source code. Never load a compiled plugin built by someone else.

Official plugins are located in the [plugin](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin)
directory. Plugin API documentation is available via [godoc](https://pkg.go.dev/codeberg.org/tslocum/sriracha#section-documentation).

## Instructions

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
in the directory are loaded.

## Compatibility

Only plugins built using the same version of Sriracha may be used.

If you attempt to load a plugin and see an error such as:

```
failed to load plugin ./plugin/fortune/fortune.so: plugin.Open("./plugin/fortune/fortune"): plugin was built with a different version of package codeberg.org/tslocum/sriracha
```

The solution is to rebuild all plugins and Sriracha itself.

## Configuration

Plugins may provide configuration options for users to set in the management panel.

The following single-value data types are supported:

- Boolean
- Integer
- Float
- Range
- Enum
- String

All data types except booleans may also have multiple values.

An example how to implement a plugin with configuration options is available in
the [Fortune](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/fortune/fortune.go) plugin.

## Events

Plugins may subscribe to receive one or more types of events by implementing
the associated event handlers. For instance, a plugin that subscribes to [Post](https://pkg.go.dev/codeberg.org/tslocum/sriracha#Post)
events would implement [PluginWithPost](https://pkg.go.dev/codeberg.org/tslocum/sriracha#PluginWithPost):

```go
type PluginWithPost interface {
	Plugin
	Post(db *Database, post *Post) error
}
```

An example of how to implement a plugin which receives new post events is
available in the [Fortune](https://codeberg.org/tslocum/sriracha/src/branch/main/plugin/fortune/fortune.go) plugin.
