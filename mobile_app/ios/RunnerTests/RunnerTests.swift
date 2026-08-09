import Flutter
import UIKit
import XCTest
@testable import Runner

class RootfsDescriptorTests: XCTestCase {
    func testValidDescriptor() {
        let url = URL(fileURLWithPath: "/tmp/test")
        let digest = String(repeating: "a", count: 64)
        let d = RootfsDescriptor(version: "3.21.0", architecture: "aarch64", digestSHA256: digest, rootfsURL: url, sourceType: .bundled)
        XCTAssertTrue(d.isValidDescriptor())
    }

    func testInvalidDigestLength() {
        let url = URL(fileURLWithPath: "/tmp/test")
        let d = RootfsDescriptor(version: "3.21.0", architecture: "aarch64", digestSHA256: "tooshort", rootfsURL: url, sourceType: .bundled)
        XCTAssertFalse(d.isValidDescriptor())
    }

    func testInvalidDigestCharacters() {
        let url = URL(fileURLWithPath: "/tmp/test")
        let digest = String(repeating: "G", count: 64)
        let d = RootfsDescriptor(version: "3.21.0", architecture: "aarch64", digestSHA256: digest, rootfsURL: url, sourceType: .bundled)
        XCTAssertFalse(d.isValidDescriptor())
    }

    func testISHFakeFSFormatConstant() {
        XCTAssertEqual(RootfsDescriptor.string(from: .ishFakeFS), "ish_fakefs")
        XCTAssertEqual(RootfsDescriptor.string(from: .unknown), "unknown")
    }

    func testStateTransitions() {
        let url = URL(fileURLWithPath: "/tmp/test")
        let digest = String(repeating: "0", count: 64)
        let d = RootfsDescriptor(version: "3.21.0", architecture: "aarch64", digestSHA256: digest, rootfsURL: url, sourceType: .bundled)
        XCTAssertEqual(d.state, .notInstalled)
        let installed = d.descriptor(bySettingState: .installed)
        XCTAssertEqual(installed.state, .installed)
    }
}

class RootfsResolverTests: XCTestCase {
    var testDir: URL!

    override func setUp() throws {
        let fm = FileManager.default
        let caches = fm.urls(for: .cachesDirectory, in: .userDomainMask).first!
        let dir = caches.appendingPathComponent("RootfsTests-\(UUID().uuidString)", isDirectory: true)
        try fm.createDirectory(at: dir, withIntermediateDirectories: true)
        testDir = dir
    }

    override func tearDown() throws {
        if testDir != nil {
            try? FileManager.default.removeItem(at: testDir)
        }
    }

    func testNoActiveManifest() {
        let resolver = RootfsResolver(baseDirectory: testDir)
        XCTAssertNil(resolver.resolveCurrentRootfs())
    }

    func testListEmptyInstalls() {
        let resolver = RootfsResolver(baseDirectory: testDir)
        let list = resolver.listInstalledRootfs()
        XCTAssertEqual(list.count, 0)
    }

    func testActiveManifestPointsToMissingVersion() {
        let activeManifestURL = testDir.appendingPathComponent("active.manifest")
        let active = ["schemaVersion": 1, "version": "3.21.0", "architecture": "aarch64"] as [String: Any]
        let data = try! JSONSerialization.data(withJSONObject: active)
        try! data.write(to: activeManifestURL, options: .atomic)

        let resolver = RootfsResolver(baseDirectory: testDir)
        XCTAssertNil(resolver.resolveCurrentRootfs())
    }

    func testIsInstalledWithValidVersion() throws {
        let resolver = RootfsResolver(baseDirectory: testDir)

        let versionDir = testDir.appendingPathComponent("versions/alpine-3.21.0-aarch64", isDirectory: true)
        let dataDir = versionDir.appendingPathComponent("data")
        let binDir = dataDir.appendingPathComponent("bin")
        let etcDir = dataDir.appendingPathComponent("etc")
        let usrDir = dataDir.appendingPathComponent("usr")

        let fm = FileManager.default
        try fm.createDirectory(at: versionDir, withIntermediateDirectories: true)
        try fm.createDirectory(at: binDir, withIntermediateDirectories: true)
        try fm.createDirectory(at: etcDir, withIntermediateDirectories: true)
        try fm.createDirectory(at: usrDir, withIntermediateDirectories: true)

        let binShPath = binDir.appendingPathComponent("sh")
        NSData().write(to: binShPath, atomically: true)

        let metaDBPath = versionDir.appendingPathComponent("meta.db")
        NSData().write(to: metaDBPath, atomically: true)

        let manifestPath = versionDir.appendingPathComponent("rootfs.manifest.json")
        let manifest: [String: Any] = [
            "schemaVersion": 1,
            "format": "ish_fakefs",
            "formatVersion": "1",
            "distribution": "alpine",
            "version": "3.21.0",
            "architecture": "aarch64",
            "packageSha256": String(repeating: "a", count: 64),
            "sourceType": "bundled"
        ]
        let manifestData = try! JSONSerialization.data(withJSONObject: manifest)
        try! manifestData.write(to: manifestPath, options: .atomic)

        XCTAssertTrue(resolver.isInstalledVersion("3.21.0", architecture: "aarch64"))

        let list = resolver.listInstalledRootfs()
        XCTAssertEqual(list.count, 1)
        XCTAssertEqual(list.first?.version, "3.21.0")

        let activeManifestURL = testDir.appendingPathComponent("active.manifest")
        let active = ["schemaVersion": 1, "version": "3.21.0", "architecture": "aarch64"] as [String: Any]
        let activeData = try! JSONSerialization.data(withJSONObject: active)
        try! activeData.write(to: activeManifestURL, options: .atomic)

        let current = resolver.resolveCurrentRootfs()
        XCTAssertNotNil(current)
        XCTAssertEqual(current?.version, "3.21.0")
        XCTAssertNotNil(current?.mountURL)
    }

    func testIsInstalledReturnsFalseForMissingVersion() {
        let resolver = RootfsResolver(baseDirectory: testDir)
        XCTAssertFalse(resolver.isInstalledVersion("3.21.0", architecture: "aarch64"))
    }
}

class RootfsIntegrityVerifierTests: XCTestCase {
    func testValidSHA256Format() {
        XCTAssertTrue(RootfsIntegrityVerifier.isValidSHA256Digest(String(repeating: "a", count: 64)))
        XCTAssertTrue(RootfsIntegrityVerifier.isValidSHA256Digest(String(repeating: "f", count: 64)))
        XCTAssertTrue(RootfsIntegrityVerifier.isValidSHA256Digest("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
        XCTAssertFalse(RootfsIntegrityVerifier.isValidSHA256Digest(""))
        XCTAssertFalse(RootfsIntegrityVerifier.isValidSHA256Digest("too-short"))
        XCTAssertFalse(RootfsIntegrityVerifier.isValidSHA256Digest(String(repeating: "G", count: 64)))
        XCTAssertFalse(RootfsIntegrityVerifier.isValidSHA256Digest("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde"))
    }

    func testSHA256Matches() {
        let testData = "hello world".data(using: .utf8)!
        let expected = "b94d27b9934d3e08a52e52d7da7dabfade356cee182"
        let result = RootfsIntegrityVerifier.verifySHA256(of: testData, againstExpected: expected)
        XCTAssertEqual(result, .match)
    }

    func testSHA256Mismatch() {
        let testData = "hello world".data(using: .utf8)!
        let result = RootfsIntegrityVerifier.verifySHA256(of: testData, againstExpected: String(repeating: "a", count: 64))
        XCTAssertEqual(result, .mismatch)
    }

    func testSHA256MissingFile() {
        let url = URL(fileURLWithPath: "/tmp/non-existent-file-\(UUID().uuidString)")
        var err: NSError?
        let result = RootfsIntegrityVerifier.verifySHA256OfFile(at: url, againstExpected: String(repeating: "a", count: 64), error: &err)
        XCTAssertEqual(result, .fileMissing)
    }
}

class RootfsInstallerTests: XCTestCase {
    var testDir: URL!

    override func setUp() throws {
        let fm = FileManager.default
        let caches = fm.urls(for: .cachesDirectory, in: .userDomainMask).first!
        let dir = caches.appendingPathComponent("RootfsInstallerTests-\(UUID().uuidString)", isDirectory: true)
        try fm.createDirectory(at: dir, withIntermediateDirectories: true)
        testDir = dir
    }

    override func tearDown() throws {
        if testDir != nil {
            try? FileManager.default.removeItem(at: testDir)
        }
    }

    func testRejectsMissingVersion() {
        let resolver = RootfsResolver(baseDirectory: testDir)
        let installer = RootfsInstaller(resolver: resolver)
        let request = RootfsInstallRequest()
        request.architecture = "aarch64"
        request.expectedDigestSHA256 = String(repeating: "a", count: 64)
        request.localBundleURL = URL(fileURLWithPath: "/tmp/fake.zip")

        let exp = expectation(description: "install completes")
        installer.installRootfs(withRequest: request, progress: nil) { success, result, error in
            XCTAssertFalse(success)
            XCTAssertNotNil(error)
            exp.fulfill()
        }
        waitForExpectations(timeout: 5)
    }

    func testRejectsInvalidDigest() {
        let resolver = RootfsResolver(baseDirectory: testDir)
        let installer = RootfsInstaller(resolver: resolver)
        let request = RootfsInstallRequest()
        request.version = "3.21.0"
        request.architecture = "aarch64"
        request.expectedDigestSHA256 = "invalid"
        request.localBundleURL = URL(fileURLWithPath: "/tmp/fake.zip")

        let exp = expectation(description: "install completes")
        installer.installRootfs(withRequest: request, progress: nil) { success, result, error in
            XCTAssertFalse(success)
            XCTAssertNotNil(error)
            exp.fulfill()
        }
        waitForExpectations(timeout: 5)
    }

    func testRejectsUnsafeVersionString() {
        let resolver = RootfsResolver(baseDirectory: testDir)
        let installer = RootfsInstaller(resolver: resolver)
        let request = RootfsInstallRequest()
        request.version = "../../evil"
        request.architecture = "aarch64"
        request.expectedDigestSHA256 = String(repeating: "a", count: 64)
        request.localBundleURL = URL(fileURLWithPath: "/tmp/fake.zip")

        let exp = expectation(description: "install completes")
        installer.installRootfs(withRequest: request, progress: nil) { success, result, error in
            XCTAssertFalse(success)
            XCTAssertNotNil(error)
            exp.fulfill()
        }
        waitForExpectations(timeout: 5)
    }

    func testRejectsHTTPRemote() {
        let resolver = RootfsResolver(baseDirectory: testDir)
        let installer = RootfsInstaller(resolver: resolver)
        let request = RootfsInstallRequest()
        request.version = "3.21.0"
        request.architecture = "aarch64"
        request.expectedDigestSHA256 = String(repeating: "a", count: 64)
        request.remoteURL = URL(string: "http://insecure.example.com/rootfs.zip")!

        let exp = expectation(description: "install completes")
        installer.installRootfs(withRequest: request, progress: nil) { success, result, error in
            XCTAssertFalse(success)
            XCTAssertNotNil(error)
            exp.fulfill()
        }
        waitForExpectations(timeout: 5)
    }
}

class RootfsArchiveExtractorTests: XCTestCase {
    func testDefaultExtractorLimits() {
        let ex = RootfsArchiveExtractor.default()
        XCTAssertTrue(ex.maxEntries > 0)
        XCTAssertTrue(ex.maxExtractedBytes > 0)
    }

    func testRejectsNonFileURL() {
        let ex = RootfsArchiveExtractor.default()
        let url = URL(string: "https://example.com/archive.zip")!
        let dest = URL(fileURLWithPath: "/tmp/test-extract-\(UUID().uuidString)")
        var err: NSError?
        let ok = ex.extractArchive(at: url, toDirectory: dest, error: &err)
        XCTAssertFalse(ok)
        XCTAssertNotNil(err)
    }
}

class IOSSandboxLifecycleTests: XCTestCase {
    let testDir: URL = {
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!
        return caches.appendingPathComponent("LifecycleTests-\(UUID().uuidString)", isDirectory: true)
    }()

    override func setUp() throws {
        try FileManager.default.createDirectory(at: testDir, withIntermediateDirectories: true)
    }

    override func tearDown() throws {
        try? FileManager.default.removeItem(at: testDir)
    }

    private func makeResolver() -> RootfsResolver {
        return RootfsResolver(baseDirectory: testDir)
    }

    private func makeBridge() -> IOSSandboxBridge {
        let resolver = makeResolver()
        return IOSSandboxBridge(resolver: resolver)
    }

    func testInitialLifecycleStateIsIdle() {
        let bridge = makeBridge()
        let health = bridge.health
        XCTAssertEqual(health.lifecycleState, .idle)
        XCTAssertEqual(health.generation, 0)
        XCTAssertFalse(health.desiredRunning)
        XCTAssertFalse(health.healthy)
    }

    func testLifecycleStateNameMapping() {
        XCTAssertEqual(kIOSSandboxLifecycleStateName[.idle], "idle")
        XCTAssertEqual(kIOSSandboxLifecycleStateName[.starting], "starting")
        XCTAssertEqual(kIOSSandboxLifecycleStateName[.running], "running")
        XCTAssertEqual(kIOSSandboxLifecycleStateName[.quiescing], "quiescing")
        XCTAssertEqual(kIOSSandboxLifecycleStateName[.quiesced], "quiesced")
        XCTAssertEqual(kIOSSandboxLifecycleStateName[.stopping], "stopping")
        XCTAssertEqual(kIOSSandboxLifecycleStateName[.failed], "failed")
    }

    func testAvailabilityMappingForLifecycleStates() {
        let bridge = makeBridge()
        XCTAssertEqual(bridge.availability, ISHAvailabilityUnavailable)
    }

    func testExecuteWhenNotRunningIsRejected() {
        let bridge = makeBridge()
        let cmd = ISHBridgeCommand()
        cmd.command = ["/bin/echo", "hello"]
        var err: NSError?
        let _ = bridge.executeCommand(cmd, error: &err)
        XCTAssertNotNil(err)
    }

    func testStopWhenIdleIsNoOp() {
        let bridge = makeBridge()
        let exp = expectation(description: "stop completes")
        bridge.stop { success, err in
            XCTAssertTrue(success)
            XCTAssertNil(err)
            exp.fulfill()
        }
        waitForExpectations(timeout: 2)
        XCTAssertEqual(bridge.health.lifecycleState, .idle)
    }

    func testRestartWhenIdleCallsCompletion() {
        let bridge = makeBridge()
        let exp = expectation(description: "restart completes")
        bridge.restart(withReason: "test") { success, err in
            XCTAssertFalse(success)
            exp.fulfill()
        }
        waitForExpectations(timeout: 2)
    }

    func testHealthDoesNotContainSensitivePaths() {
        let bridge = makeBridge()
        let health = bridge.health
        XCTAssertNil(health.activeExecutionID)
        XCTAssertNil(health.runningRootfsVersion)
        XCTAssertNil(health.runningRootfsDigest)
    }

    func testBackgroundQuiesceFromIdleDoesNotCrash() {
        let bridge = makeBridge()
        bridge.applicationDidEnterBackground()
        XCTAssertEqual(bridge.health.lifecycleState, .idle)
    }

    func testForegroundResumeFromIdleDoesNotCrash() {
        let bridge = makeBridge()
        bridge.applicationWillEnterForeground()
        XCTAssertEqual(bridge.health.lifecycleState, .idle)
    }

    func testBackgroundToForegroundPreservesDesiredRunning() {
        let bridge = makeBridge()
        XCTAssertFalse(bridge.health.desiredRunning)
        bridge.applicationDidEnterBackground()
        bridge.applicationWillEnterForeground()
        XCTAssertFalse(bridge.health.desiredRunning)
    }

    func testLifecycleStateMachineValidTransitions() {
        let bridge = makeBridge()
        XCTAssertEqual(bridge.health.lifecycleState, .idle)
        bridge.applicationDidEnterBackground()
        XCTAssertEqual(bridge.health.lifecycleState, .idle)
    }

    func testTerminateDoesNotCrash() {
        let bridge = makeBridge()
        bridge.applicationWillTerminate()
        XCTAssertEqual(bridge.health.lifecycleState, .idle)
    }

    func testAsyncStartWithoutRootfsFails() {
        let bridge = makeBridge()
        let exp = expectation(description: "start completes")
        let config = ISHBridgeConfig()
        config.rootfsURI = ""
        bridge.start(withConfig: config) { success, err in
            XCTAssertFalse(success)
            XCTAssertNotNil(err)
            exp.fulfill()
        }
        waitForExpectations(timeout: 2)
        XCTAssertEqual(bridge.health.lifecycleState, ISHSandboxLifecycleFailed)
    }
}
