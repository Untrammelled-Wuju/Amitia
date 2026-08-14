import Foundation
import BackgroundTasks

public class BackgroundTaskBridge: NSObject {
    public static let shared = BackgroundTaskBridge()

    private override init() {
        super.init()
    }
}
