#!/bin/sh
# Kindle-only atomic install/restore of the exact verified registration patch.
set -eu
mode=${1:-install}
original=50280b9576e000f483f029991acb5f08620dccb3f3b14615a3a28b80c7807e86
patched=c8371fbd1feb6a1f5bdbd01fb90f570eeb16cf20be9c3bac0eda0a1d4732f563
target=/app/KPPMainApp/js/KPPMainApp.js.hbc
backup=/mnt/us/library-home-backup/before-registration-patch.hbc
candidate=/mnt/us/library-registration-candidate.hbc
staged=$target.library-new
hash() { sha256sum "$1" | awk '{print $1}'; }
[ "$(id -u)" = 0 ] || exit 1
current=$(hash "$target")
case "$mode" in
install)
 if [ "$current" = "$patched" ]; then echo '注册检测补丁已安装，跳过。'; exit 0; fi
 [ "$current" = "$original" ] || { echo '主页文件不匹配已验证原版，拒绝安装。'; exit 1; }
 expected=$patched
 ;;
restore)
 if [ "$current" = "$original" ]; then echo '注册检测逻辑已是原版。'; exit 0; fi
 [ "$current" = "$patched" ] || { echo '当前主页含其他变更，拒绝覆盖。'; exit 1; }
 expected=$original
 ;;
*) exit 2;;
esac
[ "$(lipc-get-prop com.lab126.appmgrd activeApp 2>/dev/null)" != com.lab126.booklet.reader ] || { echo '请先退出阅读器，再安装或恢复补丁。'; exit 1; }
if grep -F " $target " /proc/mounts >/dev/null; then echo '主页文件被临时挂载，先解除临时挂载再操作。'; exit 1; fi
[ ! -e "$candidate" ] && [ ! -e "$staged" ] || { echo '发现未完成的候选文件，请先检查再重试。'; exit 1; }
mkdir -p /mnt/us/library-home-backup
writable=0
finish() {
 result=$?
 trap - 0
 rm -f "$candidate"
 if [ "$writable" = 1 ]; then rm -f "$staged"; mount -o remount,ro / || true; fi
 exit "$result"
}
trap finish 0
if [ "$mode" = install ]; then
 if [ -e "$backup" ]; then
  [ "$(hash "$backup")" = "$original" ] || { echo '已有备份不匹配，停止。'; exit 1; }
 else cp -p "$target" "$backup"; fi
 [ "$(hash "$backup")" = "$original" ] || exit 1
 sh /mnt/us/library-registration-bytepatch.sh "$target" "$candidate"
else
 found=0
 for saved in "$backup" "$target.bak" "$target.bak-final"; do
  if [ -f "$saved" ] && [ "$(hash "$saved")" = "$original" ]; then cp -p "$saved" "$candidate"; found=1; break; fi
 done
 [ "$found" = 1 ] || { echo '没有经过校验的原版备份，停止恢复。'; exit 1; }
fi
[ "$(hash "$candidate")" = "$expected" ] || exit 1
mount -o remount,rw /
writable=1
cp "$candidate" "$staged"
chown root:root "$staged"
chmod 0644 "$staged"
[ "$(hash "$staged")" = "$expected" ] || exit 1
mv "$staged" "$target"
sync
mount -o remount,ro /
writable=0
[ "$(hash "$target")" = "$expected" ] || exit 1
echo "注册检测补丁已写入：$mode；请重启 Kindle 后使新逻辑生效。"
