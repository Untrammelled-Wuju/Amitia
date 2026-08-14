import Foundation

public class SecurityScopedBookmarkStore: NSObject {
    public static let shared = SecurityScopedBookmarkStore()

    private var bookmarks: [String: Data] = [:]

    private override init() {
        super.init()
    }

    public func store(identifier: String, bookmark: Data) {
        bookmarks[identifier] = bookmark
    }

    public func resolve(identifier: String) -> Data? {
        return bookmarks[identifier]
    }

    public func remove(identifier: String) {
        bookmarks.removeValue(forKey: identifier)
    }
}
