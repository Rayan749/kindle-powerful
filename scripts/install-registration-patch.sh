#!/bin/sh
# Mac entry: install or restore the exact-file registration patch.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
mode=${1:-install}
case "$mode" in install|restore) ;; *) exit 2;; esac
sh "$root/scripts/kindle-push.sh" "$root/scripts/registration-bytepatch.sh" /mnt/us/library-registration-bytepatch.sh
sh "$root/scripts/kindle-push.sh" "$root/scripts/registration-patch-device.sh" /mnt/us/library-registration-patch.sh
sh "$root/scripts/kindle-ssh.sh" "sh /mnt/us/library-registration-patch.sh $mode"
