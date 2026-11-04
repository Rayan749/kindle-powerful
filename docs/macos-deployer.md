# macOS 部署器

支持macOS11及以上，包含Intel/Apple Silicon架构。运行 `sh scripts/build-macos-deployer.sh` 生成应用和ZIP；目标电脑无需安装Go。

填写设备当前Wi-Fi地址，用户名默认root、密码默认kindle，密码明文且可修改。点击“检查连接”，再“一键部署”。默认不勾选注册检测补丁；勾选后严格校验KPP文件哈希，匹配原版才修改，已安装跳过，不匹配停止。补丁写入需重启Kindle生效。

“恢复系统主页”移除自绘启动任务并重新选择KPP，不删除用户图书、不卸载注册检测补丁。

应用在操作结束后重置为默认密码，不保存自定义密码。主机指纹位于 `~/Library/Application Support/KindleLibraryDeployer/known_hosts`，临时部署目录结束后删除。使用临时签名，尚未Apple公证。

连接超时发生在认证之前：先确认设备醒着、当前IP、Wi-Fi SSH与同一网络；macOS15及以上在“隐私与安全性→本地网络”允许应用。Permission denied通常表示用户名/密码错误；Connection refused通常表示SSH未监听。主机指纹变更时应先核实设备身份。

前置条件可从窗口右上角查看，详见 [部署前置条件](deployment-prerequisites.md)。
