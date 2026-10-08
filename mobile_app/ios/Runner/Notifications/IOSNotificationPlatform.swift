import ActivityKit
import CallKit
import Flutter
import PushKit
import UIKit
import UserNotifications

final class IOSNotificationPlatform: NSObject, UNUserNotificationCenterDelegate, PKPushRegistryDelegate, CXProviderDelegate {
  static weak var active: IOSNotificationPlatform?

  private struct CallMetadata {
    let callId: String
    let conversationId: String
    let callerName: String
    let callType: String
  }

  private static let channelName = "com.amitia.notifications/control"
  private static let tokenKey = "amitia.notification.apns-token"
  private static let apnsInvalidatedKey = "amitia.notification.apns-invalidated"
  private static let voipTokenKey = "amitia.notification.voip-token"
  private static let voipInvalidatedKey = "amitia.notification.voip-invalidated"
  private static let initialInteractionKey = "amitia.notification.initial-interaction"

  private let channel: FlutterMethodChannel
  private weak var application: UIApplication?
  private var pushToStartTask: Task<Void, Never>?
  private var activityUpdatesTask: Task<Void, Never>?
  private var activityTokenTasks: [String: Task<Void, Never>] = [:]
  private var pushRegistry: PKPushRegistry?
  private var callProvider: CXProvider?
  private var calls: [UUID: CallMetadata] = [:]
  private var flutterReady = false

  init(messenger: FlutterBinaryMessenger) {
    self.channel = FlutterMethodChannel(
      name: Self.channelName,
      binaryMessenger: messenger
    )
    super.init()
    self.channel.setMethodCallHandler { [weak self] call, result in
      self?.handle(call, result: result)
    }
  }

  func start(application: UIApplication) {
    Self.active = self
    self.application = application
    let center = UNUserNotificationCenter.current()
    center.delegate = self
    registerCategories(center)
    application.registerForRemoteNotifications()
    configureVoIP()
    observePushToStartToken()
    observeLiveActivityUpdateTokens()
  }

  func didRegisterRemoteNotifications(deviceToken: Data) {
    let token = deviceToken.map { String(format: "%02x", $0) }.joined()
    UserDefaults.standard.set(token, forKey: Self.tokenKey)
    UserDefaults.standard.set(false, forKey: Self.apnsInvalidatedKey)
    emitToken(token)
  }

  func didFailRemoteNotifications(error: Error) {
    UserDefaults.standard.removeObject(forKey: Self.tokenKey)
    UserDefaults.standard.set(true, forKey: Self.apnsInvalidatedKey)
    channel.invokeMethod(
      "pushTokenChanged",
      arguments: [
        "provider": "apns",
        "token": NSNull(),
        "apnsTokenInvalidated": true,
      ]
    )
    channel.invokeMethod(
      "pushRegistrationFailed",
      arguments: ["message": error.localizedDescription]
    )
  }

  func handleLaunchOptions(
    _ launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) {
    guard
      let remote = launchOptions?[.remoteNotification] as? [AnyHashable: Any]
    else {
      return
    }
    storeInteraction(interaction(from: remote, source: "notificationLaunch"))
  }

  private func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
    switch call.method {
    case "initialize":
      flutterReady = true
      application?.registerForRemoteNotifications()
      capabilities { result($0) }
    case "getCapabilities":
      capabilities { result($0) }
    case "getPushToken":
      result([
        "provider": "apns",
        "token": UserDefaults.standard.string(forKey: Self.tokenKey) as Any,
      ])
    case "requestPermission":
      requestPermission(result)
    case "consumeInitialInteraction":
      result(consumeInitialInteraction())
    case "endCall":
      let arguments = call.arguments as? [String: Any] ?? [:]
      endLocalCall(
        callId: stringValue(arguments["callId"]),
        reason: stringValue(arguments["reason"])
      )
      result(true)
    case "clearExecutionNotifications":
      Task {
        await clearNotificationCategories(["AMITIA_EXECUTION"])
        if #available(iOS 16.1, *) {
          await clearExecutionActivities()
        }
        result(true)
      }
    case "clearMessageNotifications":
      Task {
        await clearNotificationCategories(["AMITIA_MESSAGE"])
        result(true)
      }
    case "clearCallNotifications":
      Task {
        await clearNotificationCategories(["AMITIA_CALL_FALLBACK"])
        await clearActiveCalls()
        result(true)
      }
    case "clearReminderNotifications":
      Task {
        await clearNotificationCategories(["AMITIA_REMINDER"])
        result(true)
      }
    case "openSettings":
      openSettings()
      result(true)
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  private func invalidatedProviders() -> [String] {
    var providers: [String] = []
    if UserDefaults.standard.bool(forKey: Self.apnsInvalidatedKey) {
      providers.append("apns")
    }
    if UserDefaults.standard.bool(forKey: Self.voipInvalidatedKey) {
      providers.append("apns-voip")
    }
    return providers
  }

  private func requestPermission(_ result: @escaping FlutterResult) {
    UNUserNotificationCenter.current().requestAuthorization(
      options: [.alert, .badge, .sound]
    ) { [weak self] granted, error in
      DispatchQueue.main.async {
        if let error {
          result(
            FlutterError(
              code: "notification_permission_failed",
              message: error.localizedDescription,
              details: nil
            )
          )
          return
        }
        if granted {
          self?.application?.registerForRemoteNotifications()
        }
        result(granted)
      }
    }
  }

  private func capabilities(_ completion: @escaping ([String: Any]) -> Void) {
    UNUserNotificationCenter.current().getNotificationSettings { settings in
      var payload: [String: Any] = [
        "platform": "ios",
        "provider": "apns",
        "appearance": UITraitCollection.current.userInterfaceStyle == .dark
          ? "dark" : "light",
        "pushConfigured": true,
        "pushToken":
          UserDefaults.standard.string(forKey: Self.tokenKey) as Any,
        "voipToken":
          UserDefaults.standard.string(forKey: Self.voipTokenKey) as Any,
        "invalidatedProviders": invalidatedProviders(),
        "notificationsEnabled":
          settings.authorizationStatus == .authorized ||
          settings.authorizationStatus == .provisional ||
          settings.authorizationStatus == .ephemeral,
        "progressStyleSupported": false,
        "communicationNotificationSupported": true,
      ]
      if #available(iOS 16.1, *) {
        let info = ActivityAuthorizationInfo()
        payload["liveActivitySupported"] = info.areActivitiesEnabled
      } else {
        payload["liveActivitySupported"] = false
      }
      if #available(iOS 17.2, *) {
        if let token = Self.currentPushToStartToken {
          payload["liveActivityPushToStartToken"] = token
        }
      }
      DispatchQueue.main.async { [weak self] in
        if #available(iOS 16.1, *) {
          payload["dynamicIslandSupported"] =
            self?.hasDynamicIslandHardware() ?? false
        } else {
          payload["dynamicIslandSupported"] = false
        }
        completion(payload)
      }
    }
  }

  private func clearNotificationCategories(
    _ categories: Set<String>
  ) async {
    guard !categories.isEmpty else { return }
    let center = UNUserNotificationCenter.current()
    let delivered = await withCheckedContinuation {
      (continuation: CheckedContinuation<[UNNotification], Never>) in
      center.getDeliveredNotifications { notifications in
        continuation.resume(returning: notifications)
      }
    }
    let deliveredIds = delivered
      .filter { categories.contains($0.request.content.categoryIdentifier) }
      .map { $0.request.identifier }
    if !deliveredIds.isEmpty {
      center.removeDeliveredNotifications(withIdentifiers: deliveredIds)
    }

    let pending = await withCheckedContinuation {
      (continuation: CheckedContinuation<[UNNotificationRequest], Never>) in
      center.getPendingNotificationRequests { requests in
        continuation.resume(returning: requests)
      }
    }
    let pendingIds = pending
      .filter { categories.contains($0.content.categoryIdentifier) }
      .map(\.identifier)
    if !pendingIds.isEmpty {
      center.removePendingNotificationRequests(withIdentifiers: pendingIds)
    }
  }

  private func clearActiveCalls() async {
    let activeCalls = calls
    calls.removeAll()
    let endedAt = Date()
    for (uuid, _) in activeCalls {
      callProvider?.reportCall(
        with: uuid,
        endedAt: endedAt,
        reason: .remoteEnded
      )
    }
  }

  @available(iOS 16.1, *)
  private func clearExecutionActivities() async {
    let now = Int64(Date().timeIntervalSince1970)
    for activity in Activity<AmitiaRunAttributes>.activities {
      let state = AmitiaRunAttributes.ContentState(
        revision: now,
        phase: "interrupted",
        title: "Amitia",
        summary: "任务系统通知已关闭",
        currentStep: 0,
        totalSteps: 0,
        progress: 0,
        updatedAt: now
      )
      await activity.end(using: state, dismissalPolicy: .immediate)
    }
  }

  @available(iOS 16.1, *)
  private func hasDynamicIslandHardware() -> Bool {
    guard UIDevice.current.userInterfaceIdiom == .phone else {
      return false
    }
    let topInset = UIApplication.shared.connectedScenes
      .compactMap { $0 as? UIWindowScene }
      .flatMap(\.windows)
      .filter { !$0.isHidden }
      .map { $0.safeAreaInsets.top }
      .max() ?? 0
    // Dynamic Island iPhones expose a larger portrait top safe-area than
    // notch-only iPhones. If no window is active yet, report false and let
    // the next capability refresh update the value.
    return topInset >= 51
  }

  private func registerCategories(_ center: UNUserNotificationCenter) {
    let reply = UNTextInputNotificationAction(
      identifier: "AMITIA_REPLY",
      title: "回复",
      options: [],
      textInputButtonTitle: "发送",
      textInputPlaceholder: "回复消息"
    )
    let answerCall = UNNotificationAction(
      identifier: "AMITIA_CALL_ANSWER",
      title: "接听",
      options: [.foreground]
    )
    let declineCall = UNNotificationAction(
      identifier: "AMITIA_CALL_DECLINE",
      title: "拒绝",
      options: [.foreground, .destructive]
    )
    center.setNotificationCategories([
      UNNotificationCategory(
        identifier: "AMITIA_MESSAGE",
        actions: [reply],
        intentIdentifiers: [],
        options: [.customDismissAction]
      ),
      UNNotificationCategory(
        identifier: "AMITIA_CALL_FALLBACK",
        actions: [answerCall, declineCall],
        intentIdentifiers: [],
        options: [.customDismissAction]
      ),
      UNNotificationCategory(
        identifier: "AMITIA_REMINDER",
        actions: [],
        intentIdentifiers: [],
        options: []
      ),
      UNNotificationCategory(
        identifier: "AMITIA_EXECUTION",
        actions: [],
        intentIdentifiers: [],
        options: []
      ),
    ])
  }

  private func openSettings() {
    guard
      let url = URL(string: UIApplication.openNotificationSettingsURLString)
    else {
      return
    }
    DispatchQueue.main.async { [weak self] in
      self?.application?.open(url)
    }
  }

  private func emitToken(_ token: String) {
    DispatchQueue.main.async { [weak self] in
      self?.channel.invokeMethod(
        "pushTokenChanged",
        arguments: ["provider": "apns", "token": token]
      )
    }
  }

  private func emitInteraction(_ payload: [String: Any]) {
    DispatchQueue.main.async { [weak self] in
      self?.channel.invokeMethod(
        "notificationInteraction",
        arguments: payload
      )
    }
  }

  private func deliverInteraction(_ payload: [String: Any]) {
    if flutterReady {
      emitInteraction(payload)
    } else {
      storeInteraction(payload)
    }
  }

  private func storeInteraction(_ payload: [String: Any]) {
    guard
      JSONSerialization.isValidJSONObject(payload),
      let data = try? JSONSerialization.data(withJSONObject: payload)
    else {
      return
    }
    UserDefaults.standard.set(data, forKey: Self.initialInteractionKey)
  }

  private func consumeInitialInteraction() -> [String: Any]? {
    guard
      let data = UserDefaults.standard.data(
        forKey: Self.initialInteractionKey
      ),
      let raw = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
    else {
      return nil
    }
    UserDefaults.standard.removeObject(forKey: Self.initialInteractionKey)
    return raw
  }

  private func interaction(
    from userInfo: [AnyHashable: Any],
    source: String,
    reply: String? = nil
  ) -> [String: Any] {
    var result: [String: Any] = ["source": source]
    for key in ["deepLink", "conversationId"] {
      if let value = userInfo[key] as? String, !value.isEmpty {
        result[key] = value
      }
    }
    if let reply, !reply.isEmpty {
      result["reply"] = reply
    }
    return result
  }

  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    willPresent notification: UNNotification,
    withCompletionHandler completionHandler:
      @escaping (UNNotificationPresentationOptions) -> Void
  ) {
    if #available(iOS 14.0, *) {
      completionHandler([.banner, .list, .sound])
    } else {
      completionHandler([.alert, .sound])
    }
  }

  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse,
    withCompletionHandler completionHandler: @escaping () -> Void
  ) {
    let userInfo = response.notification.request.content.userInfo
    if response.actionIdentifier == "AMITIA_CALL_ANSWER" ||
        response.actionIdentifier == "AMITIA_CALL_DECLINE" {
      center.removeDeliveredNotifications(
        withIdentifiers: [response.notification.request.identifier]
      )
      let action = response.actionIdentifier == "AMITIA_CALL_ANSWER"
        ? "answer"
        : "decline"
      var payload = interaction(
        from: userInfo,
        source: action == "answer" ? "callAnswer" : "callEnd"
      )
      if let deepLink = callFallbackDeepLink(from: userInfo, action: action) {
        payload["deepLink"] = deepLink
      }
      deliverInteraction(payload)
      completionHandler()
      return
    }
    let reply = (response as? UNTextInputNotificationResponse)?.userText
    let payload = interaction(
      from: userInfo,
      source: reply == nil ? "notificationTap" : "notificationReply",
      reply: reply
    )
    deliverInteraction(payload)
    completionHandler()
  }

  private func callFallbackDeepLink(
    from userInfo: [AnyHashable: Any],
    action: String
  ) -> String? {
    guard
      let raw = userInfo["deepLink"] as? String,
      var components = URLComponents(string: raw)
    else {
      return nil
    }
    var items = components.queryItems ?? []
    items.removeAll { $0.name == "action" }
    items.append(URLQueryItem(name: "action", value: action))
    components.queryItems = items
    return components.url?.absoluteString
  }

  private func configureVoIP() {
    let configuration = CXProviderConfiguration(localizedName: "Amitia")
    configuration.supportsVideo = true
    configuration.maximumCallsPerCallGroup = 1
    configuration.maximumCallGroups = 1
    configuration.supportedHandleTypes = [.generic]
    let provider = CXProvider(configuration: configuration)
    provider.setDelegate(self, queue: .main)
    callProvider = provider

    let registry = PKPushRegistry(queue: .main)
    registry.delegate = self
    registry.desiredPushTypes = [.voIP]
    pushRegistry = registry
  }

  func pushRegistry(
    _ registry: PKPushRegistry,
    didUpdate pushCredentials: PKPushCredentials,
    for type: PKPushType
  ) {
    guard type == .voIP else { return }
    let token = pushCredentials.token.map {
      String(format: "%02x", $0)
    }.joined()
    UserDefaults.standard.set(token, forKey: Self.voipTokenKey)
    UserDefaults.standard.set(false, forKey: Self.voipInvalidatedKey)
    channel.invokeMethod(
      "pushTokenChanged",
      arguments: [
        "provider": "apns",
        "token":
          UserDefaults.standard.string(forKey: Self.tokenKey) as Any,
        "voipToken": token,
      ]
    )
  }

  func pushRegistry(
    _ registry: PKPushRegistry,
    didInvalidatePushTokenFor type: PKPushType
  ) {
    guard type == .voIP else { return }
    UserDefaults.standard.removeObject(forKey: Self.voipTokenKey)
    UserDefaults.standard.set(true, forKey: Self.voipInvalidatedKey)
    channel.invokeMethod(
      "pushTokenChanged",
      arguments: [
        "provider": "apns",
        "token":
          UserDefaults.standard.string(forKey: Self.tokenKey) as Any,
        "voipTokenInvalidated": true,
      ]
    )
  }

  func pushRegistry(
    _ registry: PKPushRegistry,
    didReceiveIncomingPushWith payload: PKPushPayload,
    for type: PKPushType,
    completion: @escaping () -> Void
  ) {
    guard type == .voIP else {
      completion()
      return
    }
    let raw = payload.dictionaryPayload
    let data = raw["data"] as? [String: Any] ?? [:]
    let eventType = stringValue(raw["type"], fallback: stringValue(data["type"]))
    let callId = stringValue(raw["callId"], fallback: stringValue(data["callId"]))
    if eventType == "call.ended" {
      let rawReason = stringValue(
        raw["reason"],
        fallback: stringValue(data["reason"])
      )
      let endReason = callEndedReason(rawReason)
      let endedConversationId = stringValue(
        raw["conversationId"],
        fallback: stringValue(data["conversationId"])
      )
      if let pair = calls.first(where: { $0.value.callId == callId }) {
        callProvider?.reportCall(
          with: pair.key,
          endedAt: Date(),
          reason: endReason
        )
        calls.removeValue(forKey: pair.key)
      }
      deliverCallEndedInteraction(
        callId: callId,
        conversationId: endedConversationId,
        reason: rawReason
      )
      completion()
      return
    }

    let conversationId = stringValue(
      raw["conversationId"],
      fallback: stringValue(data["conversationId"])
    )
    guard !callId.isEmpty, !conversationId.isEmpty else {
      completion()
      return
    }
    let callerName = stringValue(
      raw["title"],
      fallback: stringValue(
        data["callerName"],
        fallback: "Amitia"
      )
    )
    let callType = stringValue(
      data["callType"],
      fallback: "audio"
    )
    if calls.values.contains(where: { $0.callId == callId }) {
      completion()
      return
    }
    let metadata = CallMetadata(
      callId: callId,
      conversationId: conversationId,
      callerName: callerName,
      callType: callType
    )
    let uuid = UUID(uuidString: callId) ?? UUID()
    calls[uuid] = metadata

    let update = CXCallUpdate()
    update.remoteHandle = CXHandle(
      type: .generic,
      value: callerName
    )
    update.localizedCallerName = callerName
    update.hasVideo = callType == "video"
    update.supportsHolding = false
    update.supportsGrouping = false
    update.supportsUngrouping = false
    callProvider?.reportNewIncomingCall(with: uuid, update: update) {
      [weak self] error in
      if error != nil {
        self?.calls.removeValue(forKey: uuid)
      }
      completion()
    }
  }

  @discardableResult
  func handleRemoteNotification(
    _ userInfo: [AnyHashable: Any],
    completion: @escaping () -> Void
  ) -> Bool {
    let data = userInfo["data"] as? [String: Any] ?? [:]
    let eventType = stringValue(
      userInfo["type"],
      fallback: stringValue(data["type"])
    )
    guard eventType == "call.ended" else {
      return false
    }
    let callId = stringValue(
      userInfo["callId"],
      fallback: stringValue(data["callId"])
    )
    guard !callId.isEmpty else {
      return false
    }

    let rawReason = stringValue(
      userInfo["reason"],
      fallback: stringValue(data["reason"])
    )
    let endReason = callEndedReason(rawReason)
    let endedConversationId = stringValue(
      userInfo["conversationId"],
      fallback: stringValue(data["conversationId"])
    )
    if let pair = calls.first(where: { $0.value.callId == callId }) {
      callProvider?.reportCall(
        with: pair.key,
        endedAt: Date(),
        reason: endReason
      )
      calls.removeValue(forKey: pair.key)
    }
    deliverCallEndedInteraction(
      callId: callId,
      conversationId: endedConversationId,
      reason: rawReason
    )

    let center = UNUserNotificationCenter.current()
    center.getDeliveredNotifications { notifications in
      let identifiers = notifications.compactMap { notification -> String? in
        let delivered = notification.request.content.userInfo
        let deliveredData = delivered["data"] as? [String: Any] ?? [:]
        let deliveredCallId = self.stringValue(
          delivered["callId"],
          fallback: self.stringValue(deliveredData["callId"])
        )
        return deliveredCallId == callId ? notification.request.identifier : nil
      }
      if !identifiers.isEmpty {
        center.removeDeliveredNotifications(withIdentifiers: identifiers)
      }
      completion()
    }
    return true
  }

  private func deliverCallEndedInteraction(
    callId: String,
    conversationId: String,
    reason: String
  ) {
    guard !callId.isEmpty else { return }
    deliverInteraction([
      "source": "callEnded",
      "callId": callId,
      "conversationId": conversationId,
      "reason": reason,
    ])
  }

  private func callEndedReason(_ rawReason: String) -> CXCallEndedReason {
    switch rawReason.trimmingCharacters(in: .whitespacesAndNewlines) {
    case "answered_elsewhere":
      return .answeredElsewhere
    case "user_declined", "declined_elsewhere":
      return .declinedElsewhere
    case "unanswered":
      return .unanswered
    case "failed", "connect_failed":
      return .failed
    default:
      return .remoteEnded
    }
  }

  private func endLocalCall(callId: String, reason: String) {
    guard !callId.isEmpty else { return }
    guard let pair = calls.first(where: { $0.value.callId == callId }) else {
      return
    }
    callProvider?.reportCall(
      with: pair.key,
      endedAt: Date(),
      reason: callEndedReason(reason)
    )
    calls.removeValue(forKey: pair.key)
  }

  func providerDidReset(_ provider: CXProvider) {
    calls.removeAll()
  }

  func provider(
    _ provider: CXProvider,
    perform action: CXAnswerCallAction
  ) {
    guard let metadata = calls[action.callUUID] else {
      action.fail()
      return
    }
    let deepLink = callDeepLink(metadata, action: "answer")
    let payload: [String: Any] = [
      "source": "callAnswer",
      "deepLink": deepLink,
      "conversationId": metadata.conversationId,
    ]
    let needsApplicationLaunch = !flutterReady
    deliverInteraction(payload)
    if needsApplicationLaunch, let url = URL(string: deepLink) {
      application?.open(url)
    }
    action.fulfill()
  }

  func provider(
    _ provider: CXProvider,
    perform action: CXEndCallAction
  ) {
    if let metadata = calls.removeValue(forKey: action.callUUID) {
      let payload: [String: Any] = [
        "source": "callEnd",
        "deepLink": callDeepLink(metadata, action: "decline"),
        "conversationId": metadata.conversationId,
      ]
      deliverInteraction(payload)
    }
    action.fulfill()
  }

  private func callDeepLink(
    _ metadata: CallMetadata,
    action: String
  ) -> String {
    var components = URLComponents()
    components.scheme = "amitia"
    components.host = "call"
    components.path = "/" + metadata.conversationId
    components.queryItems = [
      URLQueryItem(name: "call", value: metadata.callId),
      URLQueryItem(name: "type", value: metadata.callType),
      URLQueryItem(name: "caller", value: metadata.callerName),
      URLQueryItem(name: "action", value: action),
    ]
    return components.url?.absoluteString ?? ""
  }

  private func stringValue(
    _ value: Any?,
    fallback: String = ""
  ) -> String {
    if let string = value as? String {
      return string.trimmingCharacters(in: .whitespacesAndNewlines)
    }
    return fallback
  }

  func deliverLocalCall(
    type: String,
    payload: [String: Any]
  ) async -> [String: Any]? {
    let normalizedType = type.trimmingCharacters(in: .whitespacesAndNewlines)
    guard normalizedType == "call.incoming" || normalizedType == "call.ended" else {
      return nil
    }
    let callId = stringValue(payload["callId"])
    if normalizedType == "call.ended" {
      let reason = stringValue(payload["reason"])
      let conversationId = stringValue(payload["conversationId"])
      var notificationRef = ""
      if let pair = calls.first(where: { $0.value.callId == callId }) {
        callProvider?.reportCall(
          with: pair.key,
          endedAt: Date(),
          reason: callEndedReason(reason)
        )
        notificationRef = pair.key.uuidString
        calls.removeValue(forKey: pair.key)
      }
      deliverCallEndedInteraction(
        callId: callId,
        conversationId: conversationId,
        reason: reason
      )
      return notificationRef.isEmpty
        ? ["posted": false, "reason": "call_not_active"]
        : ["posted": true, "notificationRef": notificationRef]
    }

    let conversationId = stringValue(payload["conversationId"])
    guard !callId.isEmpty, !conversationId.isEmpty else {
      return [
        "posted": false,
        "reason": "call_identity_missing",
      ]
    }
    let callerName = stringValue(
      payload["callerName"],
      fallback: stringValue(payload["title"], fallback: "Amitia")
    )
    let callType = stringValue(payload["callType"], fallback: "audio")
    if let existing = calls.first(where: { $0.value.callId == callId }) {
      return [
        "posted": true,
        "notificationRef": existing.key.uuidString,
      ]
    }
    let metadata = CallMetadata(
      callId: callId,
      conversationId: conversationId,
      callerName: callerName,
      callType: callType
    )
    let uuid = UUID(uuidString: callId) ?? UUID()
    calls[uuid] = metadata
    let update = CXCallUpdate()
    update.remoteHandle = CXHandle(type: .generic, value: callerName)
    update.localizedCallerName = callerName
    update.hasVideo = callType == "video"
    update.supportsHolding = false
    update.supportsGrouping = false
    update.supportsUngrouping = false
    let reported = await withCheckedContinuation { continuation in
      callProvider?.reportNewIncomingCall(with: uuid, update: update) {
        [weak self] error in
        if error != nil {
          self?.calls.removeValue(forKey: uuid)
        }
        continuation.resume(returning: error == nil)
      }
    }
    return reported
      ? ["posted": true, "notificationRef": uuid.uuidString]
      : ["posted": false, "reason": "callkit_report_failed"]
  }

  @available(iOS 17.2, *)
  private static var currentPushToStartToken: String? {
    get {
      UserDefaults.standard.string(
        forKey: "amitia.notification.live-activity-push-to-start"
      )
    }
    set {
      UserDefaults.standard.set(
        newValue,
        forKey: "amitia.notification.live-activity-push-to-start"
      )
    }
  }

  private func observePushToStartToken() {
    guard #available(iOS 17.2, *) else {
      return
    }
    pushToStartTask?.cancel()
    pushToStartTask = Task { [weak self] in
      for await tokenData in Activity<AmitiaRunAttributes>.pushToStartTokenUpdates {
        guard !Task.isCancelled else {
          return
        }
        let token = tokenData.map { String(format: "%02x", $0) }.joined()
        Self.currentPushToStartToken = token
        await MainActor.run {
          self?.channel.invokeMethod(
            "pushTokenChanged",
            arguments: [
              "provider": "apns",
              "token":
                UserDefaults.standard.string(forKey: Self.tokenKey) as Any,
              "liveActivityPushToStartToken": token,
            ]
          )
        }
      }
    }
  }

  private func observeLiveActivityUpdateTokens() {
    guard #available(iOS 16.1, *) else {
      return
    }
    for activity in Activity<AmitiaRunAttributes>.activities {
      observeUpdateToken(for: activity)
    }
    activityUpdatesTask?.cancel()
    activityUpdatesTask = Task { [weak self] in
      for await activity in Activity<AmitiaRunAttributes>.activityUpdates {
        guard !Task.isCancelled else {
          return
        }
        await MainActor.run {
          self?.observeUpdateToken(for: activity)
        }
      }
    }
  }

  @available(iOS 16.1, *)
  @MainActor
  private func observeUpdateToken(
    for activity: Activity<AmitiaRunAttributes>
  ) {
    let activityId = activity.id
    activityTokenTasks[activityId]?.cancel()
    activityTokenTasks[activityId] = Task { [weak self] in
      for await tokenData in activity.pushTokenUpdates {
        guard !Task.isCancelled else {
          return
        }
        let token = tokenData.map { String(format: "%02x", $0) }.joined()
        self?.channel.invokeMethod(
          "liveActivityTokenChanged",
          arguments: [
            "runId": activity.attributes.runId,
            "conversationId": activity.attributes.conversationId,
            "activityId": activityId,
            "updateToken": token,
            "revision": 0,
          ]
        )
      }
    }
  }

  deinit {
    pushToStartTask?.cancel()
    activityUpdatesTask?.cancel()
    for task in activityTokenTasks.values {
      task.cancel()
    }
    channel.setMethodCallHandler(nil)
  }
}
