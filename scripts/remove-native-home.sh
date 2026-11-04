#!/bin/sh
# Restore the original KPP default and remove native-home boot jobs.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
sh "$root/scripts/kindle-ssh.sh" <<'REMOTE'
set -e
stop kindle-native-default 2>/dev/null || true
register /app/registry/kpp_home_default_app.install
lipc-set-prop com.lab126.appmgrd startdefault 0
stop kindle-library-native 2>/dev/null || true
mount -o remount,rw /
trap 'mount -o remount,ro / >/dev/null 2>&1 || true' 0
rm -f /etc/upstart/kindle-library-native.conf /etc/upstart/kindle-native-default.conf
mount -o remount,ro /
trap - 0
REMOTE
