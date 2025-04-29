# Sriracha Upgrading
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

Administrators may view current version information in the settings page.

## 1. Back everything up

Before going any further, back everything up on the server. This includes files
and PostgreSQL databases.

Store the backup somewhere other than the server, such as your computer's hard
drive. Keep this backup handy, even if the upgrade appears to be successful.

## 2. Download Sriracha

Download the [latest release](https://codeberg.org/tslocum/sriracha/releases) of
Sriracha for your platform.

## 3. Stop Sriracha

Press `Ctrl+C` in the terminal window where Sriracha is running, or send the
`SIGKILL` signal to the Sriracha server process.

## 3. Replace server binary

Replace the old `sriracha` server binary with the new one. If you are using any
plugins, replace all plugin files with updated versions.

## 4. Restart Sriracha

Database upgrades are handled automatically, regardless of the number of releases
between the old and new version. No extra commands need to be run when upgrading.

Verify no error messages are printed when Sriracha starts. If you see the usual
messages indicating Sriracha is running normally, the upgrade is complete.
