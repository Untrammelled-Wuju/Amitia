import Foundation

public struct NativeEventPayload: @unchecked Sendable {
    public let domain: String
    public let event: String
    public let timestamp: String
    public let data: [String: Any]
    public let generation: Int?
    public let priority: NativeEventPriority
    public let entityRef: String?

    public init(domain: String, event: String, timestamp: String? = nil, data: [String: Any] = [:], generation: Int? = nil, priority: NativeEventPriority = .normal, entityRef: String? = nil) {
        self.domain = domain
        self.event = event
        self.timestamp = timestamp ?? ISO8601DateFormatter().string(from: Date())
        self.data = data
        self.generation = generation
        self.priority = priority
        self.entityRef = entityRef
    }

    public func toDictionary() -> [String: Any] {
        var dict: [String: Any] = [
            "domain": domain,
            "event": event,
            "timestamp": timestamp
        ]
        if !data.isEmpty {
            dict["data"] = data
        }
        if let gen = generation {
            dict["generation"] = gen
        }
        if let ref = entityRef {
            dict["entityRef"] = ref
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

private struct DedupEntry {
    var count: Int = 1
    var lastSeenAt: Date = Date()
}

final class NativeEventEmitter {
    static let shared = NativeEventEmitter()
    private let notificationName = NSNotification.Name("com.amitia.iosnative.emitEvent")
    private let lock = NSLock()
    private var eventQueue: [NativeEventPayload] = []
    private var dedupCache: [String: DedupEntry] = [:]
    private var maxQueueSize = 100
    private var absoluteMaxQueueSize = 200
    private var maxDedupEntries = 4096
    private var dedupWindowSize: TimeInterval = 5.0
    private var dedupCleanupTimer: DispatchSourceTimer?
    private var sinks: [WeakSink] = []

    private struct WeakSink {
        weak var value: NativeEventSink?
    }

    private init() {
        startDedupCleanupTimer()
    }

    private func startDedupCleanupTimer() {
        let timer = DispatchSource.makeTimerSource(queue: DispatchQueue(label: "com.amitia.nativeevent.cleanup"))
        timer.schedule(deadline: .now() + dedupWindowSize, repeats: dedupWindowSize)
        timer.setEventHandler { [weak self] in
            self?.cleanupExpiredDedupEntries()
        }
        dedupCleanupTimer = timer
        timer.resume()
    }

    private func cleanupExpiredDedupEntries() {
        lock.lock()
        let now = Date()
        dedupCache = dedupCache.filter { _, entry in
            now.timeIntervalSince(entry.lastSeenAt) < dedupWindowSize
        }
        lock.unlock()
    }

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

        if eventQueue.count >= absoluteMaxQueueSize {
            if payload.priority <= .normal {
                lock.unlock()
                return
            }
            if let lowestIdx = eventQueue.lastIndex(where: { $0.priority <= .normal }) {
                eventQueue.remove(at: lowestIdx)
            } else if eventQueue.count >= absoluteMaxQueueSize + 50 {
                eventQueue.removeFirst()
            } else {
                eventQueue.removeFirst()
            }
        }

        let fingerprint = computeFingerprint(payload)
        let now = Date()
        let eventDedupWindow = dedupWindow(for: payload)
        if let existing = dedupCache[fingerprint] {
            if now.timeIntervalSince(existing.lastSeenAt) < eventDedupWindow {
                var updated = existing
                updated.count += 1
                updated.lastSeenAt = now
                dedupCache[fingerprint] = updated
                lock.unlock()
                return
            }
        }
        if dedupCache.count >= maxDedupEntries,
           let oldest = dedupCache.min(by: { $0.value.lastSeenAt < $1.value.lastSeenAt })?.key {
            dedupCache.removeValue(forKey: oldest)
        }
        dedupCache[fingerprint] = DedupEntry(count: 1, lastSeenAt: now)

        eventQueue.append(payload)
        eventQueue.sort { $0.priority > $1.priority }

        let sinksSnapshot = sinks
        lock.unlock()

        dispatchEvent(payload, sinks: sinksSnapshot)

        lock.lock()
        if let idx = eventQueue.lastIndex(where: { computeFingerprint($0) == fingerprint }) {
            eventQueue.remove(at: idx)
        }
        lock.unlock()
    }

    func dequeue() -> NativeEventPayload? {
        lock.lock()
        defer { lock.unlock() }

        guard !eventQueue.isEmpty else { return nil }

        let highestPriority = eventQueue.first!
        eventQueue.removeFirst()
        return highestPriority
    }

    func dequeueAll() -> [NativeEventPayload] {
        lock.lock()
        defer { lock.unlock() }

        let all = eventQueue
        eventQueue.removeAll()
        return all
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

    private func dedupWindow(for payload: NativeEventPayload) -> TimeInterval {
        if payload.event == "characteristic.value_updated" || payload.event == "characteristic.value_changed" {
            return 0.20
        }
        return dedupWindowSize
    }

    private func computeFingerprint(_ payload: NativeEventPayload) -> String {
        var components = [payload.domain, payload.event]
        if let generation = payload.generation {
            components.append("g:\(generation)")
        }
        if let ref = payload.entityRef {
            components.append("ref:\(ref)")
        }
        if let dataKey = payload.data["idempotencyKey"] as? String {
            components.append("id:\(dataKey)")
        }
        if let peripheralId = payload.data["peripheralId"] as? String {
            components.append("p:\(peripheralId)")
        }
        if let serviceUUID = payload.data["serviceUUID"] as? String {
            components.append("s:\(serviceUUID)")
        }
        if let characteristicUUID = payload.data["characteristicUUID"] as? String {
            components.append("c:\(characteristicUUID)")
        }
        if payload.event == "characteristic.value_updated" || payload.event == "characteristic.value_changed" {
            if let value = payload.data["value"] {
                components.append("v:\(String(describing: value))")
            }
        }
        return components.joined(separator: "|")
    }

    func clearDedupCache() {
        lock.lock()
        dedupCache.removeAll()
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
