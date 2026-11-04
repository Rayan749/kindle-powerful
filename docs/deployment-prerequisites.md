# 部署前置条件

1. Paperwhite 4，固件5.17.1.0.3，1072×1448/8位X11。部署器还核对已验证的framebuffer配置。
2. 用户已自行完成越狱与所需持久化安装，并完成重启。本项目不包含越狱载荷。
3. 已安装Jobman，开启Wi-Fi SSH；Mac与Kindle同一网络且可互访，无需USB。
4. SSH用户具有root权限，填写设备当前IP与实际密码。界面默认root/kindle并明文显示。
5. 保留KPP、原生阅读器、X11、Lua/liblipclua、LIPC、应用注册工具和设备字体。阅读器依赖KPP标注服务，不能停用KPP。

检查连接会核对root权限、实际版本文件 `/mnt/us/system/version.txt`、屏幕、工具、Jobman、触摸设备和字体。注册检测补丁可选，适合未注册设备的云端提示场景，不恢复云端服务或注册账户。

本项目不提供当前越狱兼容性表；越狱方式需按设备实际版本查询 [KindleModding](https://kindlemodding.org/jailbreaking/) 的最新说明，不能将本项目单台设备验证扩大为所有Paperwhite支持承诺。
