#!/bin/sh
# Run on Mac: KINDLE_PW=... sh scripts/install-native-home.sh /path/to/ARM-binary
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
[ -n "${KINDLE_PW:-}" ] || exit 2
binary=${1:?需要提供已构建的 ARM X11 二进制路径}
[ -f "$binary" ] || exit 2
sh "$root/scripts/kindle-push.sh" "$binary" /mnt/us/kindle-library-x11.next
for pair in 'tools/kindle-library/native/library-lifecycle.lua library-lifecycle.lua' 'tools/kindle-library/native/library-app.install library-app.install' 'tools/kindle-library/native/library-default.install library-default.install' 'scripts/kindle-library-native.upstart kindle-library-native.conf' 'scripts/kindle-native-default.upstart kindle-native-default.conf'; do
    set -- $pair
    sh "$root/scripts/kindle-push.sh" "$root/$1" "/mnt/us/$2"
done
sh "$root/scripts/kindle-ssh.sh" <<'REMOTE'
set -e
mkdir -p /mnt/us/library-home-backup
cp -p /app/registry/kpp_home_default_app.install /mnt/us/library-home-backup/original-default.install
# Existing processes must exit before replacing the binary.
stop kindle-native-default 2>/dev/null || true
stop kindle-library-native 2>/dev/null || true
pkill -9 -f /mnt/us/kindle-library-x11 2>/dev/null || true
pkill -9 -f /mnt/us/library-lifecycle.lua 2>/dev/null || true
[ ! -f /mnt/us/kindle-library-x11 ] || cp -p /mnt/us/kindle-library-x11 /mnt/us/library-home-backup/previous-binary
mv /mnt/us/kindle-library-x11.next /mnt/us/kindle-library-x11
chmod +x /mnt/us/kindle-library-x11
register /mnt/us/library-app.install
mount -o remount,rw /
trap 'mount -o remount,ro / >/dev/null 2>&1 || true' 0
cp /mnt/us/kindle-library-native.conf /etc/upstart/kindle-library-native.conf
cp /mnt/us/kindle-native-default.conf /etc/upstart/kindle-native-default.conf
chmod 644 /etc/upstart/kindle-library-native.conf /etc/upstart/kindle-native-default.conf
mount -o remount,ro /
trap - 0
start kindle-library-native
start kindle-native-default || true
status kindle-library-native
REMOTE
