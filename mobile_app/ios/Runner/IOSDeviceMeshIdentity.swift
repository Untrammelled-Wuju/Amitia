import CryptoKit
import Flutter
import Foundation
import Security

final class IOSDeviceMeshIdentity {
    static let channelName = "com.amitia.device_mesh/identity"

    private struct StoredIdentity: Codable {
        let deviceId: String
        let runtimeId: String
        let privateKey: Data
    }

    private let service = "com.amitia.device-mesh.identity"
    private let account = "primary"
    private var channel: FlutterMethodChannel?

    func register(messenger: FlutterBinaryMessenger) {
        let channel = FlutterMethodChannel(
            name: Self.channelName,
            binaryMessenger: messenger
        )
        self.channel = channel
        channel.setMethodCallHandler { [weak self] call, result in
            self?.handle(call: call, result: result)
        }
    }

    private func handle(call: FlutterMethodCall, result: @escaping FlutterResult) {
        do {
            switch call.method {
            case "identity":
                let identity = try loadOrCreate()
                let key = try Curve25519.Signing.PrivateKey(
                    rawRepresentation: identity.privateKey
                )
                result([
                    "deviceId": identity.deviceId,
                    "runtimeId": identity.runtimeId,
                    "platform": "ios",
                    "publicKey": base64URL(key.publicKey.rawRepresentation),
                ])
            case "sign":
                guard
                    let arguments = call.arguments as? [String: Any],
                    let raw = arguments["data"] as? String,
                    let data = Data(base64Encoded: raw)
                else {
                    result(FlutterError(
                        code: "DEVICE_MESH_SIGN_INVALID",
                        message: "signing payload is invalid",
                        details: nil
                    ))
                    return
                }
                let identity = try loadOrCreate()
                let key = try Curve25519.Signing.PrivateKey(
                    rawRepresentation: identity.privateKey
                )
                let signature = try key.signature(for: data)
                result(base64URL(signature))
            case "reset":
                try deleteIdentity()
                result(true)
            default:
                result(FlutterMethodNotImplemented)
            }
        } catch {
            result(FlutterError(
                code: "DEVICE_MESH_IDENTITY_FAILED",
                message: error.localizedDescription,
                details: nil
            ))
        }
    }

    private func loadOrCreate() throws -> StoredIdentity {
        if let data = try readKeychain() {
            let stored = try JSONDecoder().decode(StoredIdentity.self, from: data)
            _ = try Curve25519.Signing.PrivateKey(
                rawRepresentation: stored.privateKey
            )
            return stored
        }

        let key = Curve25519.Signing.PrivateKey()
        let stored = StoredIdentity(
            deviceId: "dev_" + UUID().uuidString.lowercased(),
            runtimeId: "rt_" + UUID().uuidString.lowercased(),
            privateKey: key.rawRepresentation
        )
        let encoded = try JSONEncoder().encode(stored)
        try writeKeychain(encoded)
        return stored
    }

    private func readKeychain() throws -> Data? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        if status == errSecItemNotFound {
            return nil
        }
        guard status == errSecSuccess else {
            throw NSError(
                domain: NSOSStatusErrorDomain,
                code: Int(status),
                userInfo: [NSLocalizedDescriptionKey: "Unable to read Device Mesh identity from Keychain"]
            )
        }
        return item as? Data
    }

    private func writeKeychain(_ data: Data) throws {
        try deleteIdentity(ignoreMissing: true)
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
        ]
        let status = SecItemAdd(query as CFDictionary, nil)
        guard status == errSecSuccess else {
            throw NSError(
                domain: NSOSStatusErrorDomain,
                code: Int(status),
                userInfo: [NSLocalizedDescriptionKey: "Unable to persist Device Mesh identity in Keychain"]
            )
        }
    }

    private func deleteIdentity(ignoreMissing: Bool = false) throws {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
        let status = SecItemDelete(query as CFDictionary)
        if status == errSecItemNotFound && ignoreMissing {
            return
        }
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw NSError(
                domain: NSOSStatusErrorDomain,
                code: Int(status),
                userInfo: [NSLocalizedDescriptionKey: "Unable to delete Device Mesh identity"]
            )
        }
    }

    private func base64URL(_ data: Data) -> String {
        data.base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}
