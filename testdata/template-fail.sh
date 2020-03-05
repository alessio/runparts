#!/bin/sh

progname="$(basename ${0})"

echo "${progname}: STDOUT"
echo "${progname}: STDERR" >&2

exit 1
