#!/bin/sh
set -eu

mkdir -p /run/sshd
chmod 0755 /run/sshd
exec /usr/sbin/sshd -D -e
