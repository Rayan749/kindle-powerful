#!/bin/sh
# 通过 Wi-Fi SSH 在 Kindle 上执行只读或临时命令。
# 用法：KINDLE_PW=... sh scripts/kindle-ssh.sh '远程脚本'
#   或：KINDLE_PW=... sh scripts/kindle-ssh.sh < remote-script.sh
# 密码只从环境变量读取，不写入文件。默认主机 192.168.1.100，可用 KINDLE_HOST 覆盖。
set -eu

host=${KINDLE_HOST:?请设置 Kindle 的 Wi-Fi SSH 地址}
[ -n "${KINDLE_PW:-}" ] || {
    printf '%s\n' '需要 KINDLE_PW 环境变量。' >&2
    exit 2
}
if [ "$#" -gt 0 ]; then
    script=$1
else
    script=$(cat)
fi
[ -n "$script" ] || {
    printf '%s\n' '远程脚本为空。' >&2
    exit 2
}

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(dirname -- "$script_dir")
known_hosts=$project_dir/reports/kindle_known_hosts
mkdir -p "$project_dir/reports"

b64=$(printf '%s' "$script" | base64 | tr -d '\n')
KINDLE_HOST="$host" \
KINDLE_B64="$b64" \
KINDLE_KNOWN_HOSTS="$known_hosts" \
/usr/bin/expect "$script_dir/kindle-ssh.exp"
