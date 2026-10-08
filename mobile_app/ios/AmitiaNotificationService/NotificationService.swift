import Intents
import UserNotifications

final class NotificationService: UNNotificationServiceExtension {
  private var contentHandler: ((UNNotificationContent) -> Void)?
  private var bestAttemptContent: UNMutableNotificationContent?

  override func didReceive(
    _ request: UNNotificationRequest,
    withContentHandler contentHandler: @escaping (UNNotificationContent) -> Void
  ) {
    self.contentHandler = contentHandler
    guard let mutable = request.content.mutableCopy() as? UNMutableNotificationContent else {
      contentHandler(request.content)
      return
    }
    self.bestAttemptContent = mutable

    let userInfo = request.content.userInfo
    let type = string(userInfo["type"])
    let data = userInfo["data"] as? [String: Any] ?? [:]
    let previewMode = string(data["previewMode"])
    guard type.hasPrefix("message."), previewMode != "hidden" else {
      contentHandler(mutable)
      return
    }

    let conversationId = string(userInfo["conversationId"])
    let characterId = string(userInfo["characterId"])
    let senderName = mutable.title.trimmingCharacters(in: .whitespacesAndNewlines)
    let body = mutable.body.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !conversationId.isEmpty, !senderName.isEmpty, !body.isEmpty else {
      contentHandler(mutable)
      return
    }

    let handle = INPersonHandle(
      value: characterId.isEmpty ? senderName : characterId,
      type: .unknown
    )
    let sender = INPerson(
      personHandle: handle,
      nameComponents: nil,
      displayName: senderName,
      image: nil,
      contactIdentifier: nil,
      customIdentifier: characterId.isEmpty ? senderName : characterId
    )
    let intent = INSendMessageIntent(
      recipients: nil,
      outgoingMessageType: .outgoingMessageText,
      content: body,
      speakableGroupName: nil,
      conversationIdentifier: conversationId,
      serviceName: "Amitia",
      sender: sender,
      attachments: nil
    )
    let interaction = INInteraction(intent: intent, response: nil)
    interaction.direction = .incoming
    interaction.donate { [weak self] _ in
      guard let self else {
        contentHandler(mutable)
        return
      }
      do {
        let updated = try request.content.updating(from: intent)
        if let merged = updated.mutableCopy() as? UNMutableNotificationContent {
          merged.categoryIdentifier = mutable.categoryIdentifier
          merged.threadIdentifier = mutable.threadIdentifier
          merged.userInfo = mutable.userInfo
          merged.sound = mutable.sound
          merged.badge = mutable.badge
          self.bestAttemptContent = merged
          contentHandler(merged)
        } else {
          self.bestAttemptContent = mutable
          contentHandler(mutable)
        }
      } catch {
        contentHandler(mutable)
      }
      self.contentHandler = nil
    }
  }

  override func serviceExtensionTimeWillExpire() {
    if let contentHandler, let bestAttemptContent {
      contentHandler(bestAttemptContent)
    }
    contentHandler = nil
  }

  private func string(_ value: Any?) -> String {
    (value as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
  }
}
