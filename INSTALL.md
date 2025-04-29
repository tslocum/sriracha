# Sriracha Installation
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

**Note:** The first release of Sriracha is coming soon. Stay tuned.

## 1. Create root directory

Create a directory where board files will be written to. A new sub directory is
created for each board, except when a board is created with a blank directory.
Blank directory boards are useful when your site will always only have one board.

## 2. Install PostgreSQL

Install the [PostgreSQL](https://www.postgresql.org/) database system by
following the relevant [documentation](https://www.postgresql.org/docs/17/admin.html).

## 3. Configure PostgreSQL

Create a new PostgreSQL database and role. Grant the new role access to the database
and set a password.

## 4. Download Sriracha

Download the [latest release](https://codeberg.org/tslocum/sriracha/releases) of
Sriracha for your platform. Only Linux, FreeBSD and macOS are supported.

Linux release archives include all official plugins. To use plugins on FreeBSD
or macOS, compile Sriracha and any desired plugins using the release source code.

## 5. Configure Sriracha

See [CONFIGURE.md](https://codeberg.org/tslocum/sriracha/src/branch/main/CONFIGURE.md)
for info on how to configure and run Sriracha.
