# 默认主页与恢复

X11应用ID为 `org.kindlepowerful.library`，通过系统appmgr处理load/go/pause/unload。程序内存画布负责显示；旧framebuffer后端仍保留供开发，不是默认部署路径。

设备安装文件位于 `/mnt/us/`：kindle-library-x11、library-lifecycle.lua、library-app.install、library-default.install。两个upstart任务位于 `/etc/upstart/`：kindle-library-native.conf负责常驻，kindle-native-default.conf等待LIPC后选择默认主页。

KPP保持运行提供原生阅读服务。默认主页文件 `/app/registry/kpp_home_default_app.install` 不被改写；原配置与更新前二进制保存在 `/mnt/us/library-home-backup/`。

## 恢复系统主页

设置KINDLE_HOST/KINDLE_USER/KINDLE_PW后执行 `sh scripts/remove-native-home.sh`。它重新注册原默认主页、切换主页并停止自绘任务，删除两份开机配置。自绘二进制和非默认应用注册保留，可再次安装。

## 注册检测补丁

原文件SHA-256：`50280b9576e000f483f029991acb5f08620dccb3f3b14615a3a28b80c7807e86`。
补丁版：`c8371fbd1feb6a1f5bdbd01fb90f570eeb16cf20be9c3bac0eda0a1d4732f563`。

仅支持这份确切文件。8字节差异来自KPP_Patch提交 `62d92fcded1b30d4a55f395db01a1c63c2620ecf` 的注册检测补丁。工具不携带完整HBC。写入前验证、保存原版、同目录原子替换，之后恢复根分区只读。存在临时挂载或阅读器正在前台时停止。

```sh
sh scripts/install-registration-patch.sh install
sh scripts/install-registration-patch.sh restore
```

恢复仅接受校验通过的原版备份，防止覆盖其他补丁。安装与恢复均需重启Kindle使KPP使用新文件，不自动重启设备。

## 锁屏

订阅powerd的goingToScreenSaver/exitingScreenSaver，唤醒后恢复锁屏前的自绘页；阅读器或设置前台锁屏则保持原应用。休眠时停止页眉与曝光重画，系统模态窗口在前台时不处理列表触摸。
