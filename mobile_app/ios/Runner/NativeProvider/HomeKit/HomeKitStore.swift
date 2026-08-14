import Foundation
import HomeKit

public class HomeKitStore: NSObject, HMHomeManagerDelegate {
    public static let shared = HomeKitStore()
    public let homeManager = HMHomeManager()

    private override init() {
        super.init()
    }
}
