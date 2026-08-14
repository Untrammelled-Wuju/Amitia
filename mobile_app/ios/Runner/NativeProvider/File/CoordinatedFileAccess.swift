import Foundation

public class CoordinatedFileAccess: NSObject {
    public static let shared = CoordinatedFileAccess()

    private override init() {
        super.init()
    }
}
