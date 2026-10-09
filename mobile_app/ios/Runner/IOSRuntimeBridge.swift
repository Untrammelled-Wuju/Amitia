import CryptoKit
import Flutter
import Foundation
import Security

final class IOSRuntimeBridge: NSObject, FlutterStreamHandler {
    private let queue = DispatchQueue(label: "com.amitia.runtime.lifecycle")
    private let resolver: RootfsResolver
    private let installer: RootfsInstaller
    private let identity: IOSDeviceMeshIdentity
    private var state = "NOT_INSTALLED"
    private var generation = 1
    private var profile: String?
    private var error: [String: Any]?
    private var process: IOSBusinessRuntimeProcess?
    private var host: IOSHostBridgeEndpoint?
    private var hostLease: IOSHostBridgeLease?
    private var readers: [FileHandle] = []
    private var timer: DispatchSourceTimer?
    private var token: String?
    private var startedAt = Date()
    private var foreground = true
    private var stopReason: String?
    private var sink: FlutterEventSink?

    init(resolver: RootfsResolver, installer: RootfsInstaller, identity: IOSDeviceMeshIdentity) {
        self.resolver = resolver
        self.installer = installer
        self.identity = identity
        super.init()
    }

    func register(messenger: FlutterBinaryMessenger) {
        let methods = FlutterMethodChannel(name: "com.amitia.runtime/bridge", binaryMessenger: messenger)
        methods.setMethodCallHandler { [weak self] call, result in
            guard let self else { return }
            self.queue.async {
                self.handle(call) { value in DispatchQueue.main.async { result(value) } }
            }
        }
        FlutterEventChannel(name: "com.amitia.runtime/events", binaryMessenger: messenger).setStreamHandler(self)
    }

    func onListen(withArguments arguments: Any?, eventSink events: @escaping FlutterEventSink) -> FlutterError? {
        queue.async { self.sink = events; self.emit() }
        return nil
    }

    func onCancel(withArguments arguments: Any?) -> FlutterError? {
        queue.async { self.sink = nil }
        return nil
    }

    func suspend() { queue.sync { foreground = false; stop(reason: "应用进入后台，运行中的任务已中断，请确认结果后再恢复") } }
    func resume() { queue.async { self.foreground = true; self.generation += 1; self.emit() } }
    func terminate() { suspend() }

    private func snapshot() -> [String: Any] {
        let installed = resolver.resolveCurrentRootfs() != nil
        return ["schemaVersion": 1, "state": state, "generation": generation,
                "runtimeInstalled": installed, "runtimeAvailable": state == "READY",
                "activeProfile": profile as Any? ?? NSNull(), "lastError": error as Any? ?? NSNull(),
                "manifest": manifest() as Any? ?? NSNull()]
    }

    private func manifest() -> [String: Any]? {
        guard let root = resolver.resolveCurrentRootfs() else { return nil }
        return ["schemaVersion": 1, "runtimeVersion": root.version, "packageId": "amitia-ios-runtime",
                "targetPlatform": "ios", "targetArch": "arm64", "verified": (try? verifyPayload(root)) != nil]
    }

    private func emit() {
        let value = snapshot()
        DispatchQueue.main.async { [weak self] in self?.sink?(value) }
    }

    private func handle(_ call: FlutterMethodCall, completion: @escaping (Any?) -> Void) {
        let args = call.arguments as? [String: Any] ?? [:]
        switch call.method {
        case "runtime.snapshot": completion(snapshot())
        case "runtime.manifestSummary": completion(manifest())
        case "runtime.stop": stop(reason: nil); completion(["accepted": true, "snapshot": snapshot()])
        case "runtime.start", "runtime.startWithProfile":
            let requested = args["profile"] as? String ?? "local"
            guard foreground, requested == "local" || requested == "device-agent", process == nil,
                  state != "INSTALLING" else {
                completion(["accepted": false, "snapshot": snapshot()]); return
            }
            start(requested)
            completion(["accepted": state == "STARTING", "snapshot": snapshot()])
        case "runtime.install", "runtime.reconcileEmbedded", "runtime.repair":
            install(completion: completion)
        case "runtime.verify":
            var nativeError: NSError?
            let verified = resolver.resolveCurrentRootfs().map { installer.verifyInstalledRootfs($0, error: &nativeError) && (try? verifyPayload($0)) != nil } ?? false
            if !verified { fail("VERIFY_FAILED", "iOS Runtime 校验失败") }
            completion(["accepted": verified, "snapshot": snapshot()])
        case "runtime.getBackendConnection":
            guard state == "READY", let token, process != nil,
                  args["expectedRuntimeGeneration"] == nil || (args["expectedRuntimeGeneration"] as? Int) == generation else {
                completion(["schemaVersion": 1, "status": "unavailable", "generation": generation,
                            "error": ["code": "RUNTIME_NOT_READY", "message": "iOS Runtime 尚未就绪或代次已改变"]]); return
            }
            completion(["schemaVersion": 1, "status": "available", "generation": generation,
                        "endpoint": ["host": "127.0.0.1", "port": 18899, "httpScheme": "http", "webSocketScheme": "ws", "livenessPath": "/livez", "readinessPath": "/readyz"],
                        "authentication": ["type": "local_token", "header": "X-Amitia-Local-Token", "token": token]])
        default: completion(FlutterMethodNotImplemented)
        }
    }

    private func install(completion: @escaping (Any?) -> Void) {
        guard process == nil, state != "INSTALLING" else { completion(["accepted": false, "snapshot": snapshot()]); return }
        do {
            guard let releaseURL = Bundle.main.url(forResource: "rootfs-release", withExtension: "json"),
                  let release = try JSONSerialization.jsonObject(with: Data(contentsOf: releaseURL)) as? [String: Any],
                  release["architecture"] as? String == "aarch64",
                  let version = release["version"] as? String,
                  let hash = release["sha256"] as? String, hash.count == 64,
                  let asset = release["asset"] as? String, asset == "alpine-rootfs.zip",
                  let bundle = Bundle.main.url(forResource: "alpine-rootfs", withExtension: "zip") else { throw unavailable() }
            if let installed = resolver.resolveCurrentRootfs(), installed.version == version,
               installed.packageDigestSHA256 == hash, installed.verifyISHFakeFSLayout() {
                state = "INSTALLED"; error = nil; emit(); completion(["accepted": true, "snapshot": snapshot()]); return
            }
            state = "INSTALLING"; generation += 1; error = nil; emit()
            let request = RootfsInstallRequest()
            request.version = version; request.architecture = "aarch64"; request.expectedDigestSHA256 = hash
            request.localBundleURL = bundle; request.packageFormat = "ish_fakefs"
            installer.installRootfs(withRequest: request, progress: nil) { [weak self] success, _, nativeError in
                guard let self else { return }
                self.queue.async {
                    if success { self.state = "INSTALLED"; self.error = nil; self.emit() }
                    else { self.fail("INSTALL_FAILED", nativeError?.localizedDescription ?? "iOS Runtime 安装失败") }
                    completion(["accepted": success, "snapshot": self.snapshot()])
                }
            }
        } catch { fail("SOURCE_UNAVAILABLE", "当前安装包缺少有效的 iOS Runtime，请使用包含 Runtime 的安装包"); completion(["accepted": false, "snapshot": snapshot()]) }
    }

    private func start(_ requested: String) {
        do {
            guard let root = resolver.resolveCurrentRootfs(), root.architecture == "aarch64", root.verifyISHFakeFSLayout() else { throw unavailable() }
            try verifyPayload(root)
            let hostIdentity = try identity.hostIdentity()
            guard let installation = hostIdentity["installGeneration"] as? String else { throw unavailable() }
            var volume = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
                .appendingPathComponent("AmitiaRuntimeData", isDirectory: true)
                .appendingPathComponent(installation, isDirectory: true)
                .appendingPathComponent("business-volume", isDirectory: true)
            try FileManager.default.createDirectory(at: volume, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700, .protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
            var volumeAttributes = URLResourceValues()
            volumeAttributes.isExcludedFromBackup = true
            try volume.setResourceValues(volumeAttributes)
            var random = Data(count: 32)
            guard random.withUnsafeMutableBytes({ SecRandomCopyBytes(kSecRandomDefault, 32, $0.baseAddress!) }) == errSecSuccess else { throw unavailable() }
            let localToken = random.map { String(format: "%02x", $0) }.joined()
            generation += 1
            let current = generation
            let bridgeGeneration = UUID().uuidString.lowercased()
            let runtimeRoot = "/var/lib/amitia"
            var env = ["PATH": "/opt/amitia/node/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin", "HOME": runtimeRoot + "/home",
                       "AMITIA_RUNTIME_MODE": "ios-ish", "AMITIA_RUNTIME_PLATFORM": "ios", "AMITIA_PLATFORM": "ios",
                       "AMITIA_RUNTIME_PROFILE": requested, "AMITIA_RUNTIME_ROOT": runtimeRoot,
                       "AMITIA_NODE_BIN": "/opt/amitia/node/bin/node", "AMITIA_NODE_WORK_DIR": runtimeRoot + "/workspace",
                       "AMITIA_SERVER_HOST": "127.0.0.1", "AMITIA_SERVER_PORT": "18899",
                       "AMITIA_SECURITY_MODE": "local_single_user", "AMITIA_ALLOW_REMOTE_ACCESS": "false", "AMITIA_LOCAL_TOKEN": localToken,
                       "AMITIA_IOS_HOST_BRIDGE_REQUIRED": "true", "AMITIA_IOS_HOST_BRIDGE_GENERATION": bridgeGeneration,
                       "AMITIA_GRAPH_STORE_ENABLED": "false", "AMITIA_GRAPH_STORE_REQUIRED": "false", "AMITIA_SURREAL_ENABLED": "false",
                       "AMITIA_VECTOR_STORE_ENABLED": "true", "AMITIA_QDRANT_BINARY": "/opt/amitia/qdrant/qdrant",
                       "AMITIA_QDRANT_DATA_DIR": runtimeRoot + "/data/qdrant", "AMITIA_QDRANT_HOST": "127.0.0.1", "AMITIA_QDRANT_PORT": "19178",
                       "AMITIA_SEARCH_ENABLED": "true", "AMITIA_SEARCH_DEFAULT_PROVIDER": "native", "GOMAXPROCS": "1", "GODEBUG": "asyncpreemptoff=1"]
            for directory in ["config", "data", "cache", "log", "run", "temp", "workspace"] {
                env["AMITIA_" + directory.uppercased() + "_DIR"] = runtimeRoot + "/" + directory
                env["AMITIA_" + directory.uppercased() + "_ROOT"] = runtimeRoot + "/" + directory
            }
            let config = ISHBridgeConfig()
            config.rootfsURI = root.rootfsURL.path; config.workspaceURI = "/"
            config.environment = ["AMITIA_IOS_PERSISTENT_DATA_HOST": volume.path]
            var nativeError: NSError?
            guard IOSSandboxBridge.shared().start(withConfig: config, error: &nativeError) else { throw nativeError ?? unavailable() }
            let child = IOSBusinessRuntimeProcess()
            guard child.spawn(["/opt/amitia/core/AmitiaCore"], environment: env, error: &nativeError) else { throw nativeError ?? unavailable() }
            process = child; profile = requested; token = localToken; state = "STARTING"; error = nil; startedAt = Date()
            let lease = IOSHostBridgeLease()
            hostLease = lease
            host = IOSHostBridgeEndpoint(identity: identity, generation: bridgeGeneration, descriptor: Int32(child.hostBridgeDescriptor), isCurrent: lease.isValid)
            host?.start()
            readers = [child.stdoutDescriptor, child.stderrDescriptor].map { FileHandle(fileDescriptor: Int32($0), closeOnDealloc: false) }
            for reader in readers { reader.readabilityHandler = { handle in if handle.availableData.isEmpty { handle.readabilityHandler = nil } } }
            let monitor = DispatchSource.makeTimerSource(queue: queue)
            monitor.schedule(deadline: .now(), repeating: .seconds(1))
            monitor.setEventHandler { [weak self] in self?.check(current, child: child) }
            timer = monitor; monitor.resume(); emit()
        } catch { stop(reason: nil); fail("START_FAILED", "iOS Runtime 启动失败，请检查 Runtime 安装与宿主通道") }
    }

    private func check(_ current: Int, child: IOSBusinessRuntimeProcess) {
        guard current == generation, process === child, foreground else { return }
        var nativeError: NSError?
        guard child.isRunning(&nativeError) else { stop(reason: nil); fail("CORE_EXITED", "iOS Core 已退出，当前操作状态需要重新确认"); return }
        if state == "STARTING" && Date().timeIntervalSince(startedAt) > 60 { stop(reason: nil); fail("READINESS_TIMEOUT", "iOS Core 启动超时"); return }
        var request = URLRequest(url: URL(string: "http://127.0.0.1:18899/readyz")!)
        request.timeoutInterval = 2
        request.setValue(token, forHTTPHeaderField: "X-Amitia-Local-Token")
        URLSession.shared.dataTask(with: request) { [weak self] _, response, _ in
            guard let self else { return }
            self.queue.async {
                guard current == self.generation, self.process === child, self.foreground else { return }
                if (response as? HTTPURLResponse)?.statusCode == 200 {
                    if self.state != "READY" { self.state = "READY"; self.emit() }
                } else if self.state == "READY" { self.state = "STARTING"; self.startedAt = Date(); self.emit() }
            }
        }.resume()
    }

    private func stop(reason: String?) {
        if state == "STOPPING" { return }
        stopReason = reason
        generation += 1; state = "STOPPING"; token = nil; emit()
        timer?.cancel(); timer = nil; hostLease?.invalidate(); hostLease = nil; host?.stop(); host = nil
        if let child = process {
            var nativeError: NSError?
            _ = child.cancel(&nativeError)
            reapLater(child, attempts: 0)
        }
        for reader in readers { reader.readabilityHandler = nil; try? reader.close() }
        readers.removeAll()
        if process == nil { finishStop() }
    }

    private func reapLater(_ child: IOSBusinessRuntimeProcess, attempts: Int) {
        var nativeError: NSError?
        if child.reap(&nativeError) {
            if process === child { process = nil; finishStop() }
            return
        }
        if attempts >= 100 { fail("STOP_TIMEOUT", "iOS Core 未完成退出，不能启动另一代进程"); return }
        queue.asyncAfter(deadline: .now() + .milliseconds(50)) { self.reapLater(child, attempts: attempts + 1) }
    }

    private func fail(_ code: String, _ message: String) { state = "FAILED"; error = ["code": code, "message": message, "retryable": true]; emit() }
    private func finishStop() {
        profile = nil
        if state == "FAILED" { emit(); return }
        state = resolver.resolveCurrentRootfs() == nil ? "NOT_INSTALLED" : "STOPPED"
        error = stopReason.map { ["code": "BACKGROUND_INTERRUPTED", "message": $0, "retryable": true] }
        emit()
    }

    private func verifyPayload(_ root: RootfsDescriptor) throws {
        guard root.architecture == "aarch64", root.verifyISHFakeFSLayout(),
              let path = root.manifestPath,
              let manifest = try JSONSerialization.jsonObject(with: Data(contentsOf: URL(fileURLWithPath: path))) as? [String: Any],
              manifest["guestArch"] as? String == "arm64" else { throw unavailable() }
        for (name, required) in [("core", "/opt/amitia/core/AmitiaCore"), ("node", "/opt/amitia/node/bin/node"), ("qdrant", "/opt/amitia/qdrant/qdrant")] {
            guard let entry = manifest[name] as? [String: Any], entry["path"] as? String == required,
                  let digest = entry["sha256"] as? String, digest.count == 64,
                  let expectedSize = entry["size"] as? Int, expectedSize > 0, expectedSize <= 268435456 else { throw unavailable() }
            let dataRoot = root.rootfsURL.appendingPathComponent("data").resolvingSymlinksInPath()
            let file = dataRoot.appendingPathComponent(String(required.dropFirst()))
            guard file.resolvingSymlinksInPath().path == file.path else { throw unavailable() }
            let handle = try FileHandle(forReadingFrom: file)
            defer { try? handle.close() }
            var hash = SHA256()
            var count = 0
            while let bytes = try handle.read(upToCount: 1048576), !bytes.isEmpty {
                count += bytes.count
                guard count <= expectedSize else { throw unavailable() }
                hash.update(data: bytes)
            }
            guard count == expectedSize,
                  hash.finalize().map({ String(format: "%02x", $0) }).joined() == digest else { throw unavailable() }
        }
    }
    private func unavailable() -> NSError { NSError(domain: "IOSRuntimeBridge", code: 1) }
}
