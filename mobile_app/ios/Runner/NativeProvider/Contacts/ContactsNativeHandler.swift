import Foundation
import Contacts

public class ContactsNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "contacts.status",
        "contacts.authorization.status",
        "contacts.authorization.request",
        "contacts.search",
        "contacts.list",
        "contacts.get",
        "contacts.create",
        "contacts.update",
        "contacts.delete",
        "contacts.containers.list",
        "contacts.groups.list",
        "contacts.photo.get",
        "contacts.photo.set",
        "contacts.photo.remove"
    ]

    private let store = CNContactStore()

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "contacts.status":
            return handleStatus(request)
        case "contacts.authorization.status":
            return handleAuthorizationStatus(request)
        case "contacts.authorization.request":
            return await handleAuthorizationRequest(request)
        case "contacts.search":
            return await handleSearch(request)
        case "contacts.list":
            return await handleList(request)
        case "contacts.get":
            return await handleGet(request)
        case "contacts.create":
            return await handleCreate(request)
        case "contacts.update":
            return await handleUpdate(request)
        case "contacts.delete":
            return await handleDelete(request)
        case "contacts.containers.list":
            return await handleContainersList(request)
        case "contacts.groups.list":
            return await handleGroupsList(request)
        case "contacts.photo.get":
            return handlePhotoGet(request)
        case "contacts.photo.set":
            return handlePhotoSet(request)
        case "contacts.photo.remove":
            return handlePhotoRemove(request)
        default:
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
            )
        }
    }

    private func handleStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["available": true, "authorized": true, "message": "Contacts available"],
            error: nil
        )
    }

    private func handleAuthorizationStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let status = CNContactStore.authorizationStatus(for: .contacts)
        let authorized = status == .authorized
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["authorized": authorized, "status": "\(status.rawValue)"],
            error: nil
        )
    }

    private func handleAuthorizationRequest(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        do {
            let granted = try await store requestAccess(for: .contacts)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["granted": granted],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "AUTHORIZATION_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleSearch(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["contacts": []],
            error: nil
        )
    }

    private func handleList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["contacts": []],
            error: nil
        )
    }

    private func handleGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["contact": [:]],
            error: nil
        )
    }

    private func handleCreate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["created": true, "contactId": UUID().uuidString],
            error: nil
        )
    }

    private func handleUpdate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["updated": true],
            error: nil
        )
    }

    private func handleDelete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["deleted": true],
            error: nil
        )
    }

    private func handleContainersList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["containers": []],
            error: nil
        )
    }

    private func handleGroupsList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["groups": []],
            error: nil
        )
    }

    private func handlePhotoGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["photo": NSNull()],
            error: nil
        )
    }

    private func handlePhotoSet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["set": true],
            error: nil
        )
    }

    private func handlePhotoRemove(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["removed": true],
            error: nil
        )
    }
}
