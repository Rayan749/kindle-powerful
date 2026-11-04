#!/bin/sh
# 向 Kindle 推送单个文件（scp）。密码只从 KINDLE_PW 读取，不写入文件。
# 用法：KINDLE_PW=... sh scripts/kindle-push.sh 本地文件 /mnt/us/远程名
set -eu

local=${1:?需要本地文件}
remote=${2:?需要远程路径}
[ -f "$local" ] || {
    printf '本地文件不存在：%s\n' "$local" >&2
    exit 2
}
[ -n "${KINDLE_PW:-}" ] || {
    printf '%s\n' '需要 KINDLE_PW 环境变量。' >&2
    exit 2
}

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(dirname -- "$script_dir")
known_hosts=$project_dir/reports/kindle_known_hosts
mkdir -p "$project_dir/reports"

KINDLE_LOCAL="$local" \
KINDLE_REMOTE="$remote" \
KINDLE_HOST="${KINDLE_HOST:?请设置 Kindle 的 Wi-Fi SSH 地址}" \
KINDLE_KNOWN_HOSTS="$known_hosts" \
/usr/bin/expect "$script_dir/kindle-scp.exp"
