import Foundation

public struct NativeEventPayload: Sendable {
    public let domain: String
    public let event: String
    public let data: [String: Any]
    public let generation: Int?
    public let priority: NativeEventPriority

    public init(domain: String, event: String, data: [String: Any] = [:], generation: Int? = nil, priority: NativeEventPriority = .normal) {
        self.domain = domain
        self.event = event
        self.data = data
        self.generation = generation
        self.priority = priority
    }

    public func toDictionary() -> [String: Any] {
        var dict: [String: Any] = [
            "domain": domain,
            "event": event,
            "timestamp": ISO8601DateFormatter().string(from: Date()),
            "priority": priority.rawValue
        ]
        if !data.isEmpty {
            dict["data"] = data
        }
        if let gen = generation {
            dict["generation"] = gen
        }
        return dict
    }
}

public enum NativeEventPriority: Int, Comparable, Sendable {
    case low = 0
    case normal = 1
    case high = 2
    case critical = 3

    static func < (lhs: NativeEventPriority, rhs: NativeEventPriority) -> Bool {
        return lhs.rawValue < rhs.rawValue
    }
}

public protocol NativeEventSink: AnyObject {
    func receiveEvent(_ payload: NativeEventPayload)
}

final class NativeEventEmitter {
    static let shared = NativeEventEmitter()
    private let notificationName = NSNotification.Name("com.amitia.iosnative.emitEvent")
    private let lock = NSLock()
    private var eventQueue: [NativeEventPayload] = []
    private var seenFingerprints: Set<String> = []
    private var maxQueueSize = 100
    private var sinks: [WeakSink] = []

    private struct WeakSink {
        weak var value: NativeEventSink?
    }

    private init() {}

    func registerSink(_ sink: NativeEventSink) {
        lock.lock()
        sinks.removeAll { $0.value == nil }
        sinks.append(WeakSink(value: sink))
        lock.unlock()
    }

    func unregisterSink(_ sink: NativeEventSink) {
        lock.lock()
        sinks.removeAll { $0.value === sink || $0.value == nil }
        lock.unlock()
    }

    func emit(_ payload: NativeEventPayload) {
        lock.lock()

        if eventQueue.count >= maxQueueSize && payload.priority <= .normal {
            lock.unlock()
            return
        }

        let fingerprint = computeFingerprint(payload)
        if seenFingerprints.contains(fingerprint) {
            lock.unlock()
            return
        }
        seenFingerprints.insert(fingerprint)
        if seenFingerprints.count > 500 {
            seenFingerprints.removeAll()
        }

        eventQueue.append(payload)
        eventQueue.sort { $0.priority > $1.priority }

        let snapshot = eventQueue
        let sinksSnapshot = sinks
        lock.unlock()

        for item in snapshot {
            dispatchEvent(item, sinks: sinksSnapshot)
        }
    }

    private func dispatchEvent(_ payload: NativeEventPayload, sinks: [NativeEventEmitter.WeakSink]) {
        guard let jsonData = try? JSONSerialization.data(withJSONObject: payload.toDictionary()) else {
            return
        }

        for weakSink in sinks {
            weakSink.value?.receiveEvent(payload)
        }

        NotificationCenter.default.post(
            name: notificationName,
            object: nil,
            userInfo: ["payload": jsonData]
        )
    }

    private func computeFingerprint(_ payload: NativeEventPayload) -> String {
        var components = [payload.domain, payload.event]
        if let gen = payload.generation {
            components.append("gen:\(gen)")
        }
        if let dataKey = payload.data["idempotencyKey"] as? String {
            components.append("id:\(dataKey)")
        }
        return components.joined(separator: "|")
    }

    func clearDedupCache() {
        lock.lock()
        seenFingerprints.removeAll()
        eventQueue.removeAll()
        lock.unlock()
    }

    func currentQueueDepth() -> Int {
        lock.lock()
        let count = eventQueue.count
        lock.unlock()
        return count
    }
}

extension Notification.Name {
    static let amitiaNativeEventEmitted = Notification.Name("com.amitia.iosnative.emitEvent")
}
