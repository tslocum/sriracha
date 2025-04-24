# Sriracha Board Migration
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

This application is in pre-alpha development. Don't migrate any boards yet.

Sriracha supports migrating one or more boards from [TinyIB](https://codeberg.org/tslocum/tinyib).

## Differences

### Only PostgreSQL is supported

Sriracha only supports the [PostgreSQL](https://www.postgresql.org) database system.

### Single auto-increment post ID

Sriracha uses one auto-incrementing post ID for all boards. Because of this,
migrating two more more boards will involve changing each post's ID. Reference
links inside posts are updated, but external links to old res pages will break.

### Account roles have different capabilities

See [README.md](https://codeberg.org/tslocum/sriracha/src/branch/main/README.md)
for more info on the capabilities of each role.
