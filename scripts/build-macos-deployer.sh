#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
build="$root/build/macos-deployer"
dist="$root/dist"
app="$dist/Kindle Library Deployer.app"
mkdir -p "$build" "$dist"
# Rebuild from the current source, rather than reusing an older device artifact.
(cd "$root/tools/kindle-library" && GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$build/kindle-library-x11" .)
rm -rf "$app/Contents/Resources/payload/reports"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources/payload/scripts" "$app/Contents/Resources/payload/tools/kindle-library/native"
payload="$app/Contents/Resources/payload"
cp "$build/kindle-library-x11" "$payload/kindle-library-x11"
cp "$root/tools/macos-deployer/run.sh" "$payload/run.sh"
for file in kindle-ssh.sh kindle-ssh.exp kindle-push.sh kindle-scp.exp install-native-home.sh remove-native-home.sh kindle-library-native.upstart kindle-native-default.upstart install-registration-patch.sh registration-bytepatch.sh registration-patch-device.sh; do
 cp "$root/scripts/$file" "$payload/scripts/$file"
done
cp "$root/tools/kindle-library/native/"*.lua "$payload/tools/kindle-library/native/"
cp "$root/tools/kindle-library/native/"*.install "$payload/tools/kindle-library/native/"
sdk=$(xcrun --show-sdk-path)
for arch in x86_64 arm64; do
 swiftc -O -sdk "$sdk" -target "$arch-apple-macosx11.0" "$root/tools/macos-deployer/main.swift" -o "$build/Deployer-$arch" -framework AppKit
done
lipo -create "$build/Deployer-x86_64" "$build/Deployer-arm64" -output "$app/Contents/MacOS/Deployer"
cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>Deployer</string>
<key>CFBundleIdentifier</key><string>org.kindlepowerful.deployer</string>
<key>CFBundleName</key><string>Kindle 自绘图书馆部署器</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>1.0.0</string>
<key>CFBundleVersion</key><string>2</string>
<key>LSMinimumSystemVersion</key><string>11.0</string>
<key>NSHighResolutionCapable</key><true/>
<key>NSLocalNetworkUsageDescription</key><string>通过 Wi-Fi SSH 连接你填写的 Kindle 地址，检查环境并部署或恢复图书馆主页。</string>
</dict></plist>
PLIST
cp "$root/docs/deployment-prerequisites.md" "$app/Contents/Resources/deployment-prerequisites.md"
shasum -a 256 "$payload/kindle-library-x11" > "$app/Contents/Resources/payload-sha256.txt"
cp "$root/LICENSE" "$app/Contents/Resources/LICENSE"
cp -R "$root/third-party" "$app/Contents/Resources/third-party"
codesign --force --sign - "$app"
"$app/Contents/MacOS/Deployer" --self-check
cp "$root/docs/macos-deployer.md" "$dist/使用说明.md"
cp "$root/docs/deployment-prerequisites.md" "$dist/deployment-prerequisites.md"
ditto -c -k --sequesterRsrc --keepParent "$app" "$dist/Kindle-Library-Deployer-macOS.zip"
printf 'Built: %s\n' "$app"
