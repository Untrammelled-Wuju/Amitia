import Foundation
import HomeKit

public struct HomeKitRef: Sendable {
    public let domain: String
    public let opaqueID: String
    public let generation: UInt64
    public let parentID: String?

    public init(domain: String, opaqueID: String, generation: UInt64, parentID: String? = nil) {
        self.domain = domain
        self.opaqueID = opaqueID
        self.generation = generation
        self.parentID = parentID
    }
}

public final class HomeKitStore: NSObject, HMHomeManagerDelegate, @unchecked Sendable {
    public static let shared = HomeKitStore()

    private var homeManager: HMHomeManager?
    private var generation: UInt64 = 0
    private var readinessContinuations: [CheckedContinuation<Void, Never>] = []
    private let queue = DispatchQueue(label: "com.amitia.homekit.store")

    private var cachedHomes: [HMHome] = []
    private var isInitialized = false

    private override init() {
        super.init()
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(handleWillTerminate),
            name: UIApplication.willTerminateNotification,
            object: nil
        )
    }

    @objc private func handleWillTerminate() {
        invalidate()
    }

    public func initialize() {
        queue.sync {
            if isInitialized { return }
            isInitialized = true
        }
        let manager = HMHomeManager()
        manager.delegate = self
        queue.sync {
            self.homeManager = manager
        }
    }

    public var currentGeneration: UInt64 {
        return queue.sync { generation }
    }

    public func invalidate() {
        queue.sync {
            generation += 1
            cachedHomes = []
        }
    }

    public func awaitReadiness() async {
        let isReady = queue.sync { homeManager != nil }
        if isReady { return }

        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            let shouldResume = queue.sync {
                if self.homeManager != nil {
                    return true
                }
                self.readinessContinuations.append(continuation)
                return false
            }
            if shouldResume {
                continuation.resume()
            }
        }
    }

    public func allHomes() -> [HMHome] {
        return queue.sync { cachedHomes }
    }

    public func home(withID id: String) -> HMHome? {
        return queue.sync {
            if let uuid = UUID(uuidString: id) {
                if let match = cachedHomes.first(where: { $0.uniqueIdentifier == uuid }) {
                    return match
                }
            }
            return cachedHomes.first { $0.uniqueIdentifier.uuidString == id }
        }
    }

    public func accessory(withID id: String, in home: HMHome) -> HMAccessory? {
        return home.accessories.first { $0.uniqueIdentifier.uuidString == id }
    }

    public func service(withID id: String, in accessory: HMAccessory) -> HMService? {
        return accessory.services.first { $0.uniqueIdentifier.uuidString == id }
    }

    public func service(withID id: String, homeID: String, accessoryID: String) -> HMService? {
        guard let home = home(withID: homeID) else { return nil }
        guard let accessory = accessory(withID: accessoryID, in: home) else { return nil }
        return service(withID: id, in: accessory)
    }

    public func characteristic(withID id: String, in service: HMService) -> HMCharacteristic? {
        return service.characteristics.first { $0.uniqueIdentifier.uuidString == id }
    }

    public func characteristic(withID id: String, homeID: String, accessoryID: String, serviceID: String) -> HMCharacteristic? {
        guard let service = service(withID: serviceID, homeID: homeID, accessoryID: accessoryID) else { return nil }
        return characteristic(withID: id, in: service)
    }

    public func actionSet(withID id: String, in home: HMHome) -> HMActionSet? {
        return home.actionSets.first { $0.uniqueIdentifier.uuidString == id }
    }

    public var hmHomeManager: HMHomeManager? {
        return queue.sync { homeManager }
    }

    public func homeManagerDidUpdateHomes(_ manager: HMHomeManager) {
        let continuations = queue.sync {
            self.cachedHomes = manager.homes
            let pending = self.readinessContinuations
            self.readinessContinuations = []
            return pending
        }
        for cont in continuations {
            cont.resume()
        }
    }

    public func homeManager(_ manager: HMHomeManager, didAdd home: HMHome) {
        queue.sync {
            if !cachedHomes.contains(where: { $0.uniqueIdentifier == home.uniqueIdentifier }) {
                cachedHomes.append(home)
            }
        }
    }

    public func homeManager(_ manager: HMHomeManager, didRemove home: HMHome) {
        queue.sync {
            cachedHomes.removeAll { $0.uniqueIdentifier == home.uniqueIdentifier }
        }
    }
}
