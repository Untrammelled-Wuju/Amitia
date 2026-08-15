import Foundation

public struct NativeEventPayload: Sendable {
    public let domain: String
    public let event: String
    public let data: [String: Any]

    public init(domain: String, event: String, data: [String: Any] = [:]) {
        self.domain = domain
        self.event = event
        self.data = data
    }

    public func toDictionary() -> [String: Any] {
        var dict: [String: Any] = [
            "domain": domain,
            "event": event,
            "timestamp": ISO8601DateFormatter().string(from: Date())
        ]
        if !data.isEmpty {
            dict["data"] = data
        }
        return dict
    }
}

final class NativeEventEmitter {
    static let shared = NativeEventEmitter()
    private let notificationName = NSNotification.Name("com.amitia.iosnative.emitEvent")
    private let lock = NSLock()
    private var lastEventTimestamps: [String: Date] = [:]
    private let dedupWindow: TimeInterval = 0.5

    private init() {}

    func emit(_ payload: NativeEventPayload) {
        let eventKey = "\(payload.domain).\(payload.event)"

        lock.lock()
        let now = Date()
        if let last = lastEventTimestamps[eventKey],
           now.timeIntervalSince(last) < dedupWindow {
            let shouldSuppress = shouldDedupEvent(payload)
            if shouldSuppress {
                lock.unlock()
                return
            }
        }
        lastEventTimestamps[eventKey] = now
        lock.unlock()

        guard let jsonData = try? JSONSerialization.data(withJSONObject: payload.toDictionary()) else {
            return
        }

        NotificationCenter.default.post(
            name: notificationName,
            object: nil,
            userInfo: ["payload": jsonData]
        )
    }

    private func shouldDedupEvent(_ payload: NativeEventPayload) -> Bool {
        switch payload.domain {
        case "homekit", "bluetooth":
            return true
        default:
            return false
        }
    }

    func clearDedupCache() {
        lock.lock()
        lastEventTimestamps.removeAll()
        lock.unlock()
    }
}

extension Notification.Name {
    static let amitiaNativeEventEmitted = Notification.Name("com.amitia.iosnative.emitEvent")
}
