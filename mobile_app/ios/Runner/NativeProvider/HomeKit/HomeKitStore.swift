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

public final class HomeKitStore: NSObject, HMHomeManagerDelegate, HMHomeDelegate, HMAccessoryDelegate, @unchecked Sendable {
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
        let homes = queue.sync { () -> [HMHome] in
            generation += 1
            let homes = cachedHomes
            cachedHomes = []
            return homes
        }
        for home in homes {
            home.delegate = nil
            for accessory in home.accessories {
                accessory.delegate = nil
            }
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
        for home in manager.homes {
            home.delegate = self
            for accessory in home.accessories {
                accessory.delegate = self
            }
        }
        let continuations = queue.sync {
            self.cachedHomes = manager.homes
            let pending = self.readinessContinuations
            self.readinessContinuations = []
            return pending
        }
        for cont in continuations {
            cont.resume()
        }
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "homekit",
            event: "homes.updated",
            data: ["count": manager.homes.count],
            generation: Int(currentGeneration)
        ))
    }

    public func homeManager(_ manager: HMHomeManager, didAdd home: HMHome) {
        home.delegate = self
        for accessory in home.accessories {
            accessory.delegate = self
        }
        queue.sync {
            if !cachedHomes.contains(where: { $0.uniqueIdentifier == home.uniqueIdentifier }) {
                cachedHomes.append(home)
            }
        }
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "homekit",
            event: "home.added",
            data: ["id": home.uniqueIdentifier.uuidString],
            generation: Int(currentGeneration),
            entityRef: home.uniqueIdentifier.uuidString
        ))
    }

    public func homeManager(_ manager: HMHomeManager, didRemove home: HMHome) {
        home.delegate = nil
        for accessory in home.accessories {
            accessory.delegate = nil
        }
        queue.sync {
            cachedHomes.removeAll { $0.uniqueIdentifier == home.uniqueIdentifier }
        }
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "homekit",
            event: "home.removed",
            data: ["id": home.uniqueIdentifier.uuidString],
            generation: Int(currentGeneration),
            entityRef: home.uniqueIdentifier.uuidString
        ))
    }

    // MARK: - HMHomeDelegate

    public func home(_ home: HMHome, didAdd accessory: HMAccessory) {
        accessory.delegate = self
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "homekit",
            event: "accessory.added",
            data: [
                "id": accessory.uniqueIdentifier.uuidString,
                "homeId": home.uniqueIdentifier.uuidString
            ],
            generation: Int(currentGeneration),
            entityRef: accessory.uniqueIdentifier.uuidString
        ))
    }

    public func home(_ home: HMHome, didRemove accessory: HMAccessory) {
        accessory.delegate = nil
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "homekit",
            event: "accessory.removed",
            data: [
                "id": accessory.uniqueIdentifier.uuidString,
                "homeId": home.uniqueIdentifier.uuidString
            ],
            generation: Int(currentGeneration),
            entityRef: accessory.uniqueIdentifier.uuidString
        ))
    }

    public func home(_ home: HMHome, didUpdateNameFor accessory: HMAccessory) {
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "homekit",
            event: "accessory.updated",
            data: [
                "id": accessory.uniqueIdentifier.uuidString,
                "homeId": home.uniqueIdentifier.uuidString,
                "changeType": "name"
            ],
            generation: Int(currentGeneration),
            entityRef: accessory.uniqueIdentifier.uuidString
        ))
    }

    // MARK: - HMAccessoryDelegate

    public func accessoryDidUpdateServices(_ accessory: HMAccessory) {
        let homeID = homeContaining(accessory)?.uniqueIdentifier.uuidString ?? ""
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "homekit",
            event: "service.updated",
            data: [
                "accessoryId": accessory.uniqueIdentifier.uuidString,
                "homeId": homeID,
                "serviceCount": accessory.services.count,
                "changeType": "services"
            ],
            generation: Int(currentGeneration),
            entityRef: accessory.uniqueIdentifier.uuidString
        ))
    }

    public func accessory(_ accessory: HMAccessory, didUpdateNameFor service: HMService) {
        let homeID = homeContaining(accessory)?.uniqueIdentifier.uuidString ?? ""
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "homekit",
            event: "service.updated",
            data: [
                "id": service.uniqueIdentifier.uuidString,
                "serviceType": service.serviceType,
                "accessoryId": accessory.uniqueIdentifier.uuidString,
                "homeId": homeID,
                "changeType": "name"
            ],
            generation: Int(currentGeneration),
            entityRef: accessory.uniqueIdentifier.uuidString
        ))
    }

    public func accessory(_ accessory: HMAccessory, service: HMService, didUpdateValueFor characteristic: HMCharacteristic) {
        let homeID = homeContaining(accessory)?.uniqueIdentifier.uuidString ?? ""
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "homekit",
            event: "characteristic.value_changed",
            data: [
                "id": characteristic.uniqueIdentifier.uuidString,
                "characteristicType": characteristic.characteristicType,
                "serviceId": service.uniqueIdentifier.uuidString,
                "accessoryId": accessory.uniqueIdentifier.uuidString,
                "homeId": homeID
            ],
            generation: Int(currentGeneration),
            entityRef: "\(accessory.uniqueIdentifier.uuidString)/\(service.uniqueIdentifier.uuidString)/\(characteristic.uniqueIdentifier.uuidString)"
        ))
    }

    private func homeContaining(_ accessory: HMAccessory) -> HMHome? {
        return queue.sync {
            cachedHomes.first { home in
                home.accessories.contains { $0.uniqueIdentifier == accessory.uniqueIdentifier }
            }
        }
    }

}
