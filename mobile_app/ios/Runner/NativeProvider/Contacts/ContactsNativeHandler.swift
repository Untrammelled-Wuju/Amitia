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

    public func capabilitySnapshot() -> IOSNativeCapability {
        let status = CNContactStore.authorizationStatus(for: .contacts)
        return IOSNativeCapability(
            available: true,
            authorized: status == .authorized,
            hardwareAvailable: true,
            platformSupported: true,
            foregroundRequired: false
        )
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
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
            )
        }
    }

    private func handleStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["available": true, "message": "Contacts available"],
            error: nil
        )
    }

    private func handleAuthorizationStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let status = CNContactStore.authorizationStatus(for: .contacts)
        let authorized = status == .authorized
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["authorized": authorized, "status": "\(status.rawValue)"],
            error: nil
        )
    }

    private func handleAuthorizationRequest(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        do {
            let granted = try await store.requestAccess(for: .contacts)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["granted": granted],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "AUTHORIZATION_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleSearch(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let query = request.payload?["query"] as? String, !query.isEmpty else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing search query")
            )
        }

        let keys: [CNKeyDescriptor] = [
            CNContactIdentifierKey as CNKeyDescriptor,
            CNContactGivenNameKey as CNKeyDescriptor,
            CNContactFamilyNameKey as CNKeyDescriptor,
            CNContactPhoneNumbersKey as CNKeyDescriptor,
            CNContactEmailAddressesKey as CNKeyDescriptor,
            CNContactThumbnailImageDataKey as CNKeyDescriptor
        ]

        do {
            let predicate = CNContact.predicateForContacts(matchingName: query)
            let contacts = try store.unifiedContacts(matching: predicate, keysToFetch: keys)
            let results = contacts.map { contact in
                contactToDict(contact)
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["contacts": results, "count": results.count],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "SEARCH_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let keys: [CNKeyDescriptor] = [
            CNContactIdentifierKey as CNKeyDescriptor,
            CNContactGivenNameKey as CNKeyDescriptor,
            CNContactFamilyNameKey as CNKeyDescriptor,
            CNContactPhoneNumbersKey as CNKeyDescriptor,
            CNContactEmailAddressesKey as CNKeyDescriptor,
            CNContactThumbnailImageDataKey as CNKeyDescriptor
        ]

        do {
            let fetchRequest = CNContactFetchRequest(keysToFetch: keys)
            var results: [[String: Any]] = []
            try store.enumerateContacts(with: fetchRequest) { contact, _ in
                results.append(self.contactToDict(contact))
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["contacts": results, "count": results.count],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "LIST_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let contactId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing contact id")
            )
        }

        let keys: [CNKeyDescriptor] = [
            CNContactIdentifierKey as CNKeyDescriptor,
            CNContactGivenNameKey as CNKeyDescriptor,
            CNContactFamilyNameKey as CNKeyDescriptor,
            CNContactPhoneNumbersKey as CNKeyDescriptor,
            CNContactEmailAddressesKey as CNKeyDescriptor,
            CNContactPostalAddressesKey as CNKeyDescriptor,
            CNContactBirthdayKey as CNKeyDescriptor,
            CNContactNoteKey as CNKeyDescriptor,
            CNContactThumbnailImageDataKey as CNKeyDescriptor
        ]

        do {
            let contact = try store.unifiedContact(withIdentifier: contactId, keysToFetch: keys)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["contact": contactToDict(contact)],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "contact not found: \(contactId)")
            )
        }
    }

    private func handleCreate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let contact = CNMutableContact()

        contact.givenName = request.payload?["givenName"] as? String ?? ""
        contact.familyName = request.payload?["familyName"] as? String ?? ""
        contact.note = request.payload?["note"] as? String ?? ""

        if let phoneNumbers = request.payload?["phoneNumbers"] as? [[String: Any]] {
            contact.phoneNumbers = phoneNumbers.compactMap { entry in
                guard let value = entry["value"] as? String else { return nil }
                let label = entry["label"] as? String
                return CNLabeledValue(label: label, value: CNPhoneNumber(stringValue: value))
            }
        }

        if let emailAddresses = request.payload?["emailAddresses"] as? [[String: Any]] {
            contact.emailAddresses = emailAddresses.compactMap { entry in
                guard let value = entry["value"] as? String else { return nil }
                let label = entry["label"] as? String
                return CNLabeledValue(label: label, value: NSString(string: value))
            }
        }

        let saveRequest = CNSaveRequest()
        saveRequest.add(contact, toContainerWithIdentifier: nil)

        do {
            try store.execute(saveRequest)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["created": true, "contactId": contact.identifier],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "CREATE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleUpdate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let contactId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing contact id")
            )
        }

        let keys: [CNKeyDescriptor] = [CNContactMutableKey as CNKeyDescriptor]
        do {
            let contact = try store.unifiedContact(withIdentifier: contactId, keysToFetch: keys).mutableCopy() as! CNMutableContact

            if let givenName = request.payload?["givenName"] as? String {
                contact.givenName = givenName
            }
            if let familyName = request.payload?["familyName"] as? String {
                contact.familyName = familyName
            }
            if let note = request.payload?["note"] as? String {
                contact.note = note
            }

            let saveRequest = CNSaveRequest()
            saveRequest.update(contact)

            try store.execute(saveRequest)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["updated": true, "contactId": contact.identifier],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "UPDATE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleDelete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let contactId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing contact id")
            )
        }

        let keys: [CNKeyDescriptor] = [CNContactMutableKey as CNKeyDescriptor]
        do {
            let contact = try store.unifiedContact(withIdentifier: contactId, keysToFetch: keys).mutableCopy() as! CNMutableContact

            let saveRequest = CNSaveRequest()
            saveRequest.delete(contact)

            try store.execute(saveRequest)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["deleted": true, "contactId": contactId],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "DELETE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleContainersList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        do {
            let containers = try store.containers(matching: nil)
            let results = containers.map { container in
                ["id": container.identifier, "name": container.name, "type": container.type.rawValue]
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["containers": results, "count": results.count],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "LIST_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleGroupsList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        do {
            let groups = try store.groups(matching: nil)
            let results = groups.map { group in
                ["id": group.identifier, "name": group.name]
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["groups": results, "count": results.count],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "LIST_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handlePhotoGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let contactId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing contact id")
            )
        }

        let keys: [CNKeyDescriptor] = [CNContactThumbnailImageDataKey as CNKeyDescriptor]
        do {
            let contact = try store.unifiedContact(withIdentifier: contactId, keysToFetch: keys)
            let photoData = contact.thumbnailImageData
            if let photoData = photoData {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "ok",
                    result: ["photo": photoData],
                    error: nil
                )
            } else {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "ok",
                    result: ["photo": NSNull()],
                    error: nil
                )
            }
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "contact not found: \(contactId)")
            )
        }
    }

    private func handlePhotoSet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let contactId = request.payload?["id"] as? String,
              let photoData = request.payload?["photoData"] as? Data else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing contact id or photo data")
            )
        }

        let keys: [CNKeyDescriptor] = [CNContactMutableKey as CNKeyDescriptor, CNContactImageDataKey as CNKeyDescriptor]
        do {
            let contact = try store.unifiedContact(withIdentifier: contactId, keysToFetch: keys).mutableCopy() as! CNMutableContact
            contact.imageData = photoData

            let saveRequest = CNSaveRequest()
            saveRequest.update(contact)
            try store.execute(saveRequest)

            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["set": true, "contactId": contact.identifier],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "UPDATE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handlePhotoRemove(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let contactId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing contact id")
            )
        }

        let keys: [CNKeyDescriptor] = [CNContactMutableKey as CNKeyDescriptor, CNContactImageDataKey as CNKeyDescriptor]
        do {
            let contact = try store.unifiedContact(withIdentifier: contactId, keysToFetch: keys).mutableCopy() as! CNMutableContact
            contact.imageData = nil

            let saveRequest = CNSaveRequest()
            saveRequest.update(contact)
            try store.execute(saveRequest)

            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["removed": true, "contactId": contact.identifier],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "UPDATE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func contactToDict(_ contact: CNContact) -> [String: Any] {
        var dict: [String: Any] = [:]
        dict["id"] = contact.identifier
        dict["givenName"] = contact.givenName
        dict["familyName"] = contact.familyName
        dict["fullName"] = CNContactFormatter.string(from: contact, style: .fullName) ?? ""
        dict["phoneNumbers"] = contact.phoneNumbers.map { ["label": $0.label ?? "", "value": $0.value.stringValue] }
        dict["emailAddresses"] = contact.emailAddresses.map { ["label": ($0.label ?? ""), "value": $0.value as String] }
        if let thumbnailData = contact.thumbnailImageData {
            dict["hasThumbnail"] = true
            dict["thumbnailData"] = thumbnailData
        } else {
            dict["hasThumbnail"] = false
        }
        return dict
    }
}
