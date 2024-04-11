#!/bin/sh
#
# DON'T EDIT THIS!
#
# CodeCrafters uses this file to test your code. Don't make any changes here!
#
# DON'T EDIT THIS!
set -e
tmpFile=$(mktemp)
go build -o "$tmpFile" app/*.go

# HACK: Had to use sudo to circumvent limitation on chroot, since I'm not using docker itself to run tests...
sudo "$tmpFile" "$@"
