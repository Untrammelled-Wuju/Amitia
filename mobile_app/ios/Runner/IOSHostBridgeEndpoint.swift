import Foundation
import Security
import Darwin

final class IOSHostBridgeLease {
    private let lock = NSLock()
    private var active = true
    func invalidate() { lock.lock(); active = false; lock.unlock() }
    func isValid() -> Bool { lock.lock(); defer { lock.unlock() }; return active }
}

final class IOSHostBridgeEndpoint {
    private let identity: IOSDeviceMeshIdentity
    private let generation: String
    private let input: FileHandle
    private let output: FileHandle
    private let isCurrent: () -> Bool
    private let queue = DispatchQueue(label: "com.amitia.runtime.host-bridge")
    private let lock = NSRecursiveLock()
    private var active = true
    private var pending = Data()
    private var sequence: UInt64 = 0

    init(identity: IOSDeviceMeshIdentity, generation: String, descriptor: Int32, isCurrent: @escaping () -> Bool) {
        self.identity = identity
        self.generation = generation
        self.input = FileHandle(fileDescriptor: descriptor, closeOnDealloc: false)
        self.output = input
        self.isCurrent = isCurrent
    }

    func start() {
        input.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            self?.queue.async { [weak self] in self?.consume(data) }
        }
    }

    func stop() {
        lock.lock()
        active = false
        lock.unlock()
        input.readabilityHandler = nil
        try? input.close()
    }

    private func valid() -> Bool {
        lock.lock()
        let value = active
        lock.unlock()
        return value && isCurrent()
    }

    private func consume(_ data: Data) {
        lock.lock()
        defer { lock.unlock() }
        guard valid(), !data.isEmpty else { stop(); return }
        pending.append(data)
        guard pending.count <= 131072 else { stop(); return }
        while let newline = pending.firstIndex(of: 10) {
            let line = pending.prefix(upTo: newline)
            pending.removeSubrange(...newline)
            guard valid() else { return }
            do {
                guard let request = try JSONSerialization.jsonObject(with: line) as? [String: Any],
                      (request["schemaVersion"] as? Int) == 1,
                      let id = request["id"] as? String, !id.isEmpty, id.count <= 128,
                      (request["generation"] as? String) == generation,
                      let method = request["method"] as? String,
                      let next = UInt64(id), String(next) == id, next == sequence + 1 else { stop(); return }
                sequence = next
                var response: [String: Any] = ["schemaVersion": 1, "id": id, "generation": generation]
                do {
                    response["result"] = try execute(method, request["params"] as? [String: Any] ?? [:])
                } catch {
                    response["error"] = ["code": "HOST_OPERATION_FAILED", "message": "iOS 宿主操作未完成"]
                }
                guard valid() else { return }
                var encoded = try JSONSerialization.data(withJSONObject: response)
                encoded.append(10)
                try output.write(contentsOf: encoded)
            } catch { stop(); return }
        }
    }

    private func execute(_ method: String, _ params: [String: Any]) throws -> [String: Any] {
        guard valid() else { throw failure() }
        switch method {
        case "network.privateAddresses":
            var interfaces: UnsafeMutablePointer<ifaddrs>?
            guard getifaddrs(&interfaces) == 0 else { throw failure() }
            defer { freeifaddrs(interfaces) }
            var result = Set<String>()
            var current = interfaces
            while let entry = current {
                defer { current = entry.pointee.ifa_next }
                guard let address = entry.pointee.ifa_addr,
                      entry.pointee.ifa_flags & UInt32(IFF_UP) != 0,
                      entry.pointee.ifa_flags & UInt32(IFF_LOOPBACK) == 0,
                      address.pointee.sa_family == UInt8(AF_INET) else { continue }
                var buffer = [CChar](repeating: 0, count: Int(NI_MAXHOST))
                guard getnameinfo(address, socklen_t(address.pointee.sa_len), &buffer, socklen_t(buffer.count), nil, 0, NI_NUMERICHOST) == 0 else { continue }
                let value = String(cString: buffer).lowercased()
                let parts = value.split(separator: ".").compactMap { Int($0) }
                let privateIPv4 = parts.count == 4 && parts.allSatisfy({ $0 >= 0 && $0 <= 255 }) &&
                    (parts[0] == 10 || parts[0] == 192 && parts[1] == 168 || parts[0] == 172 && (16...31).contains(parts[1]))
                if privateIPv4 { result.insert(value) }
                if result.count == 16 { break }
            }
            return ["addresses": result.sorted()]
        case "identity.get":
            return try identity.hostIdentity()
        case "identity.sign":
            guard let raw = params["data"] as? String, raw.count <= 87384,
                  let data = Data(base64Encoded: raw), data.base64EncodedString() == raw else { throw failure() }
            return ["signature": try identity.hostSign(data)]
        case "secret.get", "secret.set", "secret.delete":
            guard let key = params["key"] as? String, key.count == 64,
                  key.allSatisfy({ "0123456789abcdef".contains($0) }),
                  let installation = try identity.hostIdentity()["installGeneration"] as? String else { throw failure() }
            var query: [String: Any] = [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrService as String: "com.amitia.runtime.secrets." + installation,
                kSecAttrAccount as String: key,
            ]
            if method == "secret.get" {
                query[kSecReturnData as String] = true
                query[kSecMatchLimit as String] = kSecMatchLimitOne
                var item: CFTypeRef?
                let status = SecItemCopyMatching(query as CFDictionary, &item)
                if status == errSecItemNotFound { return ["found": false, "data": ""] }
                guard status == errSecSuccess, let data = item as? Data else { throw failure() }
                return ["found": true, "data": data.base64EncodedString()]
            }
            if method == "secret.delete" {
                let status = SecItemDelete(query as CFDictionary)
                guard status == errSecSuccess || status == errSecItemNotFound else { throw failure() }
                return ["ok": true]
            }
            guard let raw = params["data"] as? String, raw.count <= 87384,
                  let data = Data(base64Encoded: raw), data.count <= 65536,
                  data.base64EncodedString() == raw, valid() else { throw failure() }
            let updates: [String: Any] = [kSecValueData as String: data]
            var status = SecItemUpdate(query as CFDictionary, updates as CFDictionary)
            if status == errSecItemNotFound {
                query[kSecValueData as String] = data
                query[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
                status = SecItemAdd(query as CFDictionary, nil)
            }
            guard status == errSecSuccess else { throw failure() }
            return ["ok": true]
        default:
            throw failure()
        }
    }

    private func failure() -> NSError { NSError(domain: "IOSHostBridge", code: 1) }
}
