import Foundation

@objc public protocol IOSNativeTransportDelegate: AnyObject {
    func transportDidBecomeReady(_ transport: IOSNativeTransport)
    func transportDidBecomeUnready(_ transport: IOSNativeTransport)
}

@objc public class IOSNativeTransport: NSObject {

    private weak var delegate: IOSNativeTransportDelegate?
    private var host: IOSNativeHost?
    private var isReady: Bool = false
    private var transportGeneration: UInt64 = 0
    private let queue = DispatchQueue(label: "com.amitia.iosnative.transport")

    public init(host: IOSNativeHost, delegate: IOSNativeTransportDelegate?) {
        self.host = host
        self.delegate = delegate
        super.init()
    }

    public func attach() {
        queue.async { [weak self] in
            guard let self = self else { return }
            self.performHandshake()
        }
    }

    public func detach() {
        queue.async { [weak self] in
            guard let self = self else { return }
            self.isReady = false
            self.delegate?.transportDidBecomeUnready(self)
        }
    }

    public func sendRequest(_ request: IOSNativeRequest, completion: @escaping (IOSNativeResponse) -> Void) {
        queue.async { [weak self] in
            guard let self = self, let host = self.host else {
                let error = IOSNativeError(code: "TRANSPORT_UNAVAILABLE", message: "transport not available")
                let response = IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "error",
                    result: nil,
                    error: error
                )
                completion(response)
                return
            }

            Task {
                let response = await host.execute(request)
                completion(response)
            }
        }
    }

    public var transportReady: Bool {
        return queue.sync { isReady }
    }

    public var currentGeneration: UInt64 {
        return queue.sync { transportGeneration }
    }

    private func performHandshake() {
        guard let host = host else {
            isReady = false
            delegate?.transportDidBecomeUnready(self)
            return
        }

        let handshake = host.handshake()
        guard let platform = handshake["platform"] as? String, platform == "ios" else {
            isReady = false
            delegate?.transportDidBecomeUnready(self)
            return
        }

        guard handshake["protocolVersion"] != nil else {
            isReady = false
            delegate?.transportDidBecomeUnready(self)
            return
        }

        queue.async(flags: .barrier) { [weak self] in
            self?.isReady = true
            self?.transportGeneration += 1
        }
        delegate?.transportDidBecomeReady(self)
    }

    public func reconnect() {
        queue.async { [weak self] in
            guard let self = self else { return }
            self.isReady = false
            self.host?.invalidateTransientRefs()
            self.performHandshake()
        }
    }
}
