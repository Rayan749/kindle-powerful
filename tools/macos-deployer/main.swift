import AppKit

final class App: NSObject, NSApplicationDelegate, NSWindowDelegate {
    var window: NSWindow!
    let host = NSTextField(string: "")
    let user = NSTextField(string: "root")
    let password = NSTextField(string: "kindle")
    let output = NSTextView()
    let registrationPatch = NSButton(checkboxWithTitle: "可选：安装注册检测补丁，屏蔽未注册设备的云端提示", target: nil, action: nil)
    let status = NSTextField(labelWithString: "填写 Kindle 的 Wi-Fi SSH 连接信息")
    var buttons: [NSButton] = []
    var running: Process?
    var pipe: Pipe?
    var work: URL?
    func applicationDidFinishLaunching(_ notification: Notification) {
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 700, height: 610), styleMask: [.titled, .closable, .miniaturizable], backing: .buffered, defer: false)
        window.title = "Kindle 自绘图书馆部署器"
        window.center()
        window.delegate = self
        let view = window.contentView!
        func label(_ text: String, _ y: CGFloat, size: CGFloat = 13) {
            let l = NSTextField(labelWithString: text)
            l.font = .systemFont(ofSize: size, weight: size > 20 ? .semibold : .regular)
            l.frame = NSRect(x: 30, y: y, width: 640, height: size > 20 ? 34 : 40)
            view.addSubview(l)
        }
        label("Kindle 自绘图书馆", 550, size: 25)
        let prerequisites = NSButton(title: "前置条件", target: self, action: #selector(showPrerequisites))
        prerequisites.bezelStyle = .rounded
        prerequisites.frame = NSRect(x: 545, y: 550, width: 125, height: 32)
        view.addSubview(prerequisites)
        label("Wi-Fi SSH 一键部署 · 已越狱的 Paperwhite 4 · 固件 5.17.1.0.3", 510)
        let fields: [(String, NSTextField, CGFloat)] = [("设备 IP / 主机名", host, 466), ("SSH 用户名", user, 422), ("SSH 密码", password, 378)]
        for (name, field, y) in fields {
            let l = NSTextField(labelWithString: name)
            l.frame = NSRect(x: 30, y: y + 4, width: 150, height: 22)
            view.addSubview(l)
            field.frame = NSRect(x: 185, y: y, width: 485, height: 30)
            field.font = .systemFont(ofSize: 15)
            view.addSubview(field)
        }
        host.placeholderString = "例如 192.168.1.100"
        password.placeholderString = "仅用于本次连接，不保存"
        registrationPatch.state = .off
        registrationPatch.frame = NSRect(x: 30, y: 335, width: 640, height: 26)
        view.addSubview(registrationPatch)
        let titles = ["检查连接", "一键部署", "恢复系统主页"]
        for (i, title) in titles.enumerated() {
            let b = NSButton(title: title, target: self, action: #selector(action(_:)))
            b.tag = i; b.bezelStyle = .rounded
            b.frame = NSRect(x: CGFloat(30 + i * 215), y: 298, width: 200, height: 34)
            view.addSubview(b); buttons.append(b)
        }
        status.frame = NSRect(x: 30, y: 262, width: 640, height: 28)
        view.addSubview(status)
        let scroll = NSScrollView(frame: NSRect(x: 30, y: 52, width: 640, height: 205))
        scroll.hasVerticalScroller = true; scroll.borderType = .bezelBorder
        output.isEditable = false; output.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
        output.isVerticallyResizable = true
        output.autoresizingMask = [.width]
        output.frame = scroll.contentView.bounds
        output.minSize = NSSize(width: 0, height: scroll.contentSize.height)
        output.maxSize = NSSize(width: CGFloat.greatestFiniteMagnitude, height: CGFloat.greatestFiniteMagnitude)
        output.textContainer?.widthTracksTextView = true
        scroll.documentView = output; view.addSubview(scroll)
        label("先在“越狱管理 → SSH & USB Networking”开启 Wi-Fi SSH；Mac 与 Kindle 接入同一网络。", 5, size: 11)
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
    @objc func showPrerequisites() {
        let alert = NSAlert()
        alert.messageText = "部署前置条件"
        alert.informativeText = "完成设备越狱与 Wi-Fi SSH 配置后，再进行一键部署。"
        let scroll = NSScrollView(frame: NSRect(x: 0, y: 0, width: 570, height: 340))
        scroll.hasVerticalScroller = true
        let text = NSTextView(frame: scroll.contentView.bounds)
        text.isEditable = false
        text.font = .systemFont(ofSize: 13)
        text.isVerticallyResizable = true
        text.autoresizingMask = [.width]
        text.textContainer?.widthTracksTextView = true
        if let path = Bundle.main.url(forResource: "deployment-prerequisites", withExtension: "md"), let body = try? String(contentsOf: path, encoding: .utf8) {
            text.string = body.replacingOccurrences(of: "# 部署前置条件\n", with: "")
        } else { text.string = "请先完成越狱，安装 Jobman 并开启 Wi-Fi SSH。当前仅支持 PW4 / 5.17.1.0.3。" }
        scroll.documentView = text
        alert.accessoryView = scroll
        alert.addButton(withTitle: "知道了")
        alert.runModal()
    }
    @objc func action(_ sender: NSButton) {
        guard running == nil else { return }
        let h = host.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
        let u = user.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
        let safeHost = h.range(of: "^[A-Za-z0-9][A-Za-z0-9.-]*$", options: .regularExpression) != nil
        let safeUser = u.range(of: "^[A-Za-z_][A-Za-z0-9_-]*$", options: .regularExpression) != nil
        guard safeHost && safeUser && !password.stringValue.isEmpty else {
            status.stringValue = "请填写有效的设备地址、用户名和密码。"; return
        }
        let operation = ["check", "install", "restore"][sender.tag]
        do {
            let support = try FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true).appendingPathComponent("KindleLibraryDeployer")
            try FileManager.default.createDirectory(at: support, withIntermediateDirectories: true)
            let directory = support.appendingPathComponent(UUID().uuidString)
            try FileManager.default.copyItem(at: Bundle.main.resourceURL!.appendingPathComponent("payload"), to: directory)
            work = directory
            let reports = directory.appendingPathComponent("reports")
            try FileManager.default.createDirectory(at: reports, withIntermediateDirectories: true)
            let known = support.appendingPathComponent("known_hosts")
            if FileManager.default.fileExists(atPath: known.path) {
                try FileManager.default.copyItem(at: known, to: reports.appendingPathComponent("kindle_known_hosts"))
            }
            let p = Process(); let stream = Pipe()
            p.executableURL = URL(fileURLWithPath: "/bin/sh")
            p.arguments = [directory.appendingPathComponent("run.sh").path, operation]
            var env = ProcessInfo.processInfo.environment
            env["KINDLE_INSTALL_REGISTRATION_PATCH"] = registrationPatch.state == .on ? "1" : "0"
            env["KINDLE_HOST"] = h; env["KINDLE_USER"] = u; env["KINDLE_PW"] = password.stringValue
            p.environment = env
            p.standardOutput = stream; p.standardError = stream
            output.string = ""; status.stringValue = "正在执行，请保持设备在线…"
            stream.fileHandleForReading.readabilityHandler = { [weak self] handle in
                let data = handle.availableData
                guard !data.isEmpty else { return }
                let text = String(decoding: data, as: UTF8.self)
                DispatchQueue.main.async { self?.append(text) }
            }
            p.terminationHandler = { [weak self] process in
                DispatchQueue.main.async {
                    guard let self = self else { return }
                    stream.fileHandleForReading.readabilityHandler = nil
                    self.append(String(decoding: stream.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self))
                    if process.terminationStatus == 0 {
                        self.status.stringValue = "操作完成"
                    } else if self.output.string.contains("Operation timed out") || self.output.string.contains("No route to host") || self.output.string.contains("ssh timeout") {
                        self.status.stringValue = "网络连接失败：请确认当前 IP、Wi-Fi SSH 和本地网络权限。"
                        self.append("\n请唤醒 Kindle，在 Jobman 重新开启 Wi-Fi SSH 并核对 IP。macOS 15 及以上还需在系统设置 → 隐私与安全性 → 本地网络允许本应用。\n")
                    } else if self.output.string.contains("Permission denied") || self.output.string.contains("Authentication failed") {
                        self.status.stringValue = "SSH 认证失败：请检查用户名和密码。"
                    } else if self.output.string.contains("Connection refused") {
                        self.status.stringValue = "设备拒绝 SSH 连接：请在 Jobman 开启 Wi-Fi SSH。"
                    } else {
                        self.status.stringValue = "环境检查或操作失败，请查看日志中的具体原因。"
                    }
                    let source = reports.appendingPathComponent("kindle_known_hosts")
                    if FileManager.default.fileExists(atPath: source.path) {
                        try? FileManager.default.removeItem(at: known)
                        try? FileManager.default.copyItem(at: source, to: known)
                    }
                    try? FileManager.default.removeItem(at: directory)
                    self.running = nil; self.pipe = nil; self.password.stringValue = "kindle"
                    self.buttons.forEach { $0.isEnabled = true }
                    self.registrationPatch.isEnabled = true
                }
            }
            running = p; pipe = stream
            try p.run()
            password.stringValue = ""
            buttons.forEach { $0.isEnabled = false }
            registrationPatch.isEnabled = false
        } catch {
            running = nil; pipe = nil
            if let directory = work { try? FileManager.default.removeItem(at: directory) }
            status.stringValue = "无法启动：\(error.localizedDescription)"
        }
    }
    func append(_ text: String) {
        output.textStorage?.append(NSAttributedString(string: text))
        output.scrollToEndOfDocument(nil)
    }
    func windowShouldClose(_ sender: NSWindow) -> Bool {
        if running != nil { status.stringValue = "当前操作正在执行，请完成后再关闭。"; return false }; return true
    }
    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        if running != nil {
            let alert = NSAlert(); alert.messageText = "部署正在执行"
            alert.informativeText = "请等待当前操作结束后再退出，避免安装中断。"
            alert.runModal(); return .terminateCancel
        }
        return .terminateNow
    }
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }
}
if CommandLine.arguments.contains("--self-check") {
    guard Bundle.main.url(forResource: "deployment-prerequisites", withExtension: "md") != nil else { exit(1) }
    let base = Bundle.main.resourceURL!.appendingPathComponent("payload")
    for file in ["run.sh", "kindle-library-x11", "scripts/install-native-home.sh", "scripts/remove-native-home.sh", "scripts/kindle-ssh.exp", "scripts/kindle-scp.exp", "scripts/registration-bytepatch.sh", "scripts/registration-patch-device.sh", "tools/kindle-library/native/library-lifecycle.lua"] {
        guard FileManager.default.fileExists(atPath: base.appendingPathComponent(file).path) else { fputs("Missing payload: \(file)\n", stderr); exit(1) }
    }
    print("Payload OK"); exit(0)
}
let app = NSApplication.shared
let delegate = App()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
