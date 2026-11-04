#!/bin/sh
# Exact-file delta for KPP_Patch registration detection, pinned and verified.
# Upstream: KindleModding/KPP_Patch 62d92fcded1b30d4a55f395db01a1c63c2620ecf
# Only supports original SHA-256 below; output must match the verified patched SHA.
set -eu
input=${1:?需要原始 HBC 路径}
output=${2:?需要新文件路径}
[ "$input" != "$output" ] || exit 2
hash() {
 if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}
[ "$(hash "$input")" = 50280b9576e000f483f029991acb5f08620dccb3f3b14615a3a28b80c7807e86 ] || { echo '主页文件不匹配，停止生成补丁。' >&2; exit 1; }
[ ! -e "$output" ] || { echo '候选文件已存在。' >&2; exit 1; }
cp "$input" "$output"
printf '\004' | dd of="$output" bs=1 seek=86996 conv=notrunc 2>/dev/null
printf '\004' | dd of="$output" bs=1 seek=135604 conv=notrunc 2>/dev/null
printf '\162' | dd of="$output" bs=1 seek=1615559 conv=notrunc 2>/dev/null
printf '\130' | dd of="$output" bs=1 seek=1615561 conv=notrunc 2>/dev/null
printf '\000' | dd of="$output" bs=1 seek=1615562 conv=notrunc 2>/dev/null
printf '\160' | dd of="$output" bs=1 seek=2052939 conv=notrunc 2>/dev/null
printf '\130' | dd of="$output" bs=1 seek=2052941 conv=notrunc 2>/dev/null
printf '\000' | dd of="$output" bs=1 seek=2052942 conv=notrunc 2>/dev/null
[ "$(hash "$output")" = c8371fbd1feb6a1f5bdbd01fb90f570eeb16cf20be9c3bac0eda0a1d4732f563 ] || { rm -f "$output"; echo '补丁输出校验失败。' >&2; exit 1; }
echo '注册检测补丁候选校验通过。'
