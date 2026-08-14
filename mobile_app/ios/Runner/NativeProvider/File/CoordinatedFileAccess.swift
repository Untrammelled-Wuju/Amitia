import Foundation

public class CoordinatedFileAccess: NSObject {
    public static let shared = CoordinatedFileAccess()

    private override init() {
        super.init()
    }

    public func read(from url: URL, completion: @escaping (Data?, Error?) -> Void) {
        let coordinator = NSFileCoordinator(filePresenter: nil)
        var coordinatorError: NSError?
        var readError: NSError?
        var resultData: Data?

        coordinator.coordinate(readingItemAt: url, options: .withoutChanges, error: &coordinatorError) { url in
            do {
                resultData = try Data(contentsOf: url)
            } catch {
                readError = error as NSError
            }
        }

        if let error = coordinatorError ?? readError {
            completion(nil, error)
        } else {
            completion(resultData, nil)
        }
    }

    public func write(_ data: Data, to url: URL, completion: @escaping (Bool, Error?) -> Void) {
        let coordinator = NSFileCoordinator(filePresenter: nil)
        var coordinatorError: NSError?
        var writeError: NSError?

        coordinator.coordinate(writingItemAt: url, options: .forReplacing, error: &coordinatorError) { url in
            do {
                try data.write(to: url, options: .atomic)
            } catch {
                writeError = error as NSError
            }
        }

        if let error = coordinatorError ?? writeError {
            completion(false, error)
        } else {
            completion(true, nil)
        }
    }

    public func move(from sourceURL: URL, to destURL: URL, completion: @escaping (Bool, Error?) -> Void) {
        let coordinator = NSFileCoordinator(filePresenter: nil)
        var coordinatorError: NSError?
        var moveError: NSError?

        coordinator.coordinate(writingItemAt: sourceURL, options: .forMoving, writingItemAt: destURL, options: .forReplacing, error: &coordinatorError) { source, dest in
            do {
                try FileManager.default.moveItem(at: source, to: dest)
            } catch {
                moveError = error as NSError
            }
        }

        if let error = coordinatorError ?? moveError {
            completion(false, error)
        } else {
            completion(true, nil)
        }
    }

    public func copy(from sourceURL: URL, to destURL: URL, completion: @escaping (Bool, Error?) -> Void) {
        let coordinator = NSFileCoordinator(filePresenter: nil)
        var coordinatorError: NSError?
        var copyError: NSError?

        coordinator.coordinate(writingItemAt: sourceURL, options: .forReading, writingItemAt: destURL, options: .forReplacing, error: &coordinatorError) { source, dest in
            do {
                try FileManager.default.copyItem(at: source, to: dest)
            } catch {
                copyError = error as NSError
            }
        }

        if let error = coordinatorError ?? copyError {
            completion(false, error)
        } else {
            completion(true, nil)
        }
    }

    public func delete(_ url: URL, completion: @escaping (Bool, Error?) -> Void) {
        let coordinator = NSFileCoordinator(filePresenter: nil)
        var coordinatorError: NSError?
        var deleteError: NSError?

        coordinator.coordinate(writingItemAt: url, options: .forDeleting, error: &coordinatorError) { url in
            do {
                try FileManager.default.removeItem(at: url)
            } catch {
                deleteError = error as NSError
            }
        }

        if let error = coordinatorError ?? deleteError {
            completion(false, error)
        } else {
            completion(true, nil)
        }
    }
}
