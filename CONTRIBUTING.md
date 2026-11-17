# Contributing to Sriracha

**Please read and follow these instructions before contributing.**

Only the [latest tagged release](https://codeberg.org/tslocum/sriracha/releases)
of Sriracha is supported. If you are [building](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md#build-from-source)
Sriracha from source, you must checkout and build the latest release tag only.
The main branch is unstable. Building Sriracha from the main branch will result
in file and/or database corruption.

The [Sriracha Manual](https://codeberg.org/tslocum/sriracha/src/branch/main/MANUAL.md)
contains helpful information that might allow you to resolve the issue yourself.
Please read the manual in its entirety before opening an issue or pull request.

Support is only available for installations that do not override templates
without the `extra_` prefix. If you are using custom templates, try starting
Sriracha without them. If the error persists without any custom templates,
you may open an issue.

Please include the version of Sriracha you are running in the description of
your issue. Disclose whether you are running an official tagged release archive,
official Docker image or building Sriracha from source. If you are building
from source, specify which commit hash you are building.

Official and unofficial support is also available via [/help/](https://sriracha.rocket9labs.com/help/)
and [Matrix](https://matrix.to/#/#sriracha:matrix.org).
