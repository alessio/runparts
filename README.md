[![Travis-CI Status](https://api.travis-ci.org/alessio/runparts.png?branch=master)](http://travis-ci.org/#!/alessio/runparts)
# runparts
Run scripts or programs in a directory.

This is a Go implementation of the `run-parts` command
shipped with the Debian [debianutils package](https://tracker.debian.org/pkg/debianutils).

The original program is written in C. Its source code can be found [here](https://salsa.debian.org/debian/debianutils/-/tree/master).

This implementation aims to be as compatible as possible with the original program
shipped with the Debian distribution's original package. This program has not been
tested on Windows systems.
# Installation

Just

```
go get github.com/alessio/runparts
```
