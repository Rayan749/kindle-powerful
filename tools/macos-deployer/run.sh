#!/bin/sh
set -eu
action=${1:?missing action}
root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
export KINDLE_HOST KINDLE_USER KINDLE_PW
case "$action" in check|install|restore) ;; *) exit 2;; esac
if [ "$action" = restore ]; then
 sh "$root/scripts/kindle-ssh.sh" 'test "$(id -u)" = 0 && test -f /app/registry/kpp_home_default_app.install'
 sh "$root/scripts/remove-native-home.sh"
 printf '已恢复系统默认主页。\n'
 exit 0
fi
printf '正在连接 Kindle…\n'
sh "$root/scripts/kindle-ssh.sh" <<'REMOTE'
set -e
[ "$(id -u)" = 0 ] || { echo '需要具有 root 权限的 SSH 用户。'; exit 1; }
[ -f /app/registry/kpp_home_default_app.install ] || { echo '未找到兼容的 KPP 环境。'; exit 1; }
for tool in /usr/bin/lua /usr/bin/register /usr/bin/lipc-get-prop /usr/bin/lipc-set-prop /usr/bin/lipc-probe /usr/bin/xrefresh /sbin/start /sbin/stop; do
 [ -x "$tool" ] || { echo "缺少系统工具：$tool"; exit 1; }
done
/usr/bin/lua -e 'require("liblipclua")' || { echo '缺少 Lua LIPC 绑定。'; exit 1; }
[ -x /mnt/us/jobman/jobman ] || { echo '请先安装 Jobman 并开启 Wi-Fi SSH。'; exit 1; }
[ -r /dev/input/event2 ] || { echo '未找到可读取的触摸设备。'; exit 1; }
firmware=$(cat /mnt/us/system/version.txt 2>/dev/null || true)
case "$firmware" in
 'Kindle 5.17.1.0.3 ('*) printf 'SSH 已连接，设备版本：%s\n' "$firmware";;
 *) echo '当前包仅适配固件 5.17.1.0.3，未识别到兼容的设备版本。'; exit 1;;
esac
[ "$(cat /sys/class/graphics/fb0/virtual_size)" = '1088,6144' ] || { echo '当前包仅适配已验证的 PW4 屏幕配置。'; exit 1; }
for face in STHeitiMedium.ttf STHeitiBold.ttf STSongMedium.ttf; do
 [ -f "/usr/java/lib/fonts/$face" ] || { echo "缺少设备字体：$face"; exit 1; }
done
echo '连接与环境检查通过。'
REMOTE
case "$action" in
check) exit 0;;
install)
 if [ "${KINDLE_INSTALL_REGISTRATION_PATCH:-0}" = 1 ]; then
  printf '正在安装可选注册检测补丁…\n'
  sh "$root/scripts/install-registration-patch.sh" install
 fi
 printf '正在部署当前版本…\n'
 sh "$root/scripts/install-native-home.sh" "$root/kindle-library-x11"
 printf '正在检查默认主页…\n'
 sh "$root/scripts/kindle-ssh.sh" <<'REMOTE'
set -e
i=0
while [ "$i" -lt 30 ]; do
 if [ "$(lipc-get-prop com.lab126.appmgrd activeApp 2>/dev/null)" = org.kindlepowerful.library ]; then
  status kindle-library-native
  echo '部署成功：默认主页已进入自绘图书馆。'
  exit 0
 fi
 sleep 1
 i=$((i+1))
done
echo '安装任务已执行，但默认主页未在等待时间内进入，请查看日志。'
exit 1
REMOTE
 ;;
restore)
 printf '正在恢复系统主页…\n'
 sh "$root/scripts/remove-native-home.sh"
 printf '已恢复系统默认主页。\n'
 ;;
*) exit 2;;
esac
