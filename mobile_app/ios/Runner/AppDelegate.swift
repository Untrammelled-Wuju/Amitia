import Flutter
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate, IOSNativeTransportDelegate {
  private var sandboxHandler: IOSSandboxMethodHandler?
  private var rootfsHandler: RootfsInstallMethodHandler?
  private var iosNativeHost: IOSNativeHost?
  private var nativeTransport: IOSNativeTransport?
  private var notificationPlatform: IOSNotificationPlatform?
  private var deviceMeshIdentity: IOSDeviceMeshIdentity?
  private var runtimeBridge: IOSRuntimeBridge?

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    GeneratedPluginRegistrant.register(with: self)

    let notificationRegistrar = self.registrar(
      forPlugin: "IOSNotificationPlatform"
    )!
    self.notificationPlatform = IOSNotificationPlatform(
      messenger: notificationRegistrar.messenger()
    )
    self.notificationPlatform?.start(application: application)
    self.notificationPlatform?.handleLaunchOptions(launchOptions)

    let deviceMeshRegistrar = self.registrar(
      forPlugin: "IOSDeviceMeshIdentity"
    )!
    self.deviceMeshIdentity = IOSDeviceMeshIdentity()
    self.deviceMeshIdentity?.register(
      messenger: deviceMeshRegistrar.messenger()
    )

    let bridge = IOSSandboxBridge.shared()
    self.sandboxHandler = IOSSandboxMethodHandler(bridge: bridge)
    self.sandboxHandler?.register(with: self.registrar(forPlugin: "IOSSandboxBridge")!)

    self.iosNativeHost = IOSNativeHost()
    self.iosNativeHost?.registerHandler(HealthKitNativeHandler())
    self.iosNativeHost?.registerHandler(CalendarNativeHandler())
    self.iosNativeHost?.registerHandler(RemindersNativeHandler())
    self.iosNativeHost?.registerHandler(ContactsNativeHandler())
    self.iosNativeHost?.registerHandler(HomeKitNativeHandler())
    self.iosNativeHost?.registerHandler(BluetoothNativeHandler())
    self.iosNativeHost?.registerHandler(ClipboardNativeHandler())
    self.iosNativeHost?.registerHandler(MediaNativeHandler())
    self.iosNativeHost?.registerHandler(AlarmNativeHandler())
    self.iosNativeHost?.registerHandler(ShareNativeHandler())
    self.iosNativeHost?.registerHandler(ShortcutNativeHandler())
    self.iosNativeHost?.registerHandler(BackgroundNativeHandler())
    self.iosNativeHost?.registerHandler(FileNativeHandler())
    self.iosNativeHost?.registerHandler(IOSLocalNotificationNativeHandler())
    self.iosNativeHost?.registerHandler(IOSDeviceTimeNativeHandler())
    self.iosNativeHost?.registerHandler(IOSScreenAwakeNativeHandler())

    if let host = self.iosNativeHost {
      self.nativeTransport = IOSNativeTransport(host: host, delegate: self)
      self.nativeTransport?.attach()
      let nativeRegistrar = self.registrar(forPlugin: "IOSNativeBridgePlugin")!
      let nativeMessenger = nativeRegistrar.messenger()
      AudioRecorderAdapter.shared.registerRealtimeChannels(messenger: nativeMessenger)
      RealtimeVisualAdapter.shared.registerRealtimeChannels(messenger: nativeMessenger)
      IOSNativeBridgePlugin.register(
        messenger: nativeMessenger,
        host: host
      )

      let dispatcher = BackendActionDispatcherImpl.shared
      dispatcher.configure(messenger: nativeMessenger)
      ShortcutActionGateway.shared.setupBackendDispatcher(dispatcher)
      BackgroundNativeHandler.registerBGTaskHandlers()
    }

    let resolver = RootfsResolver()
    let installer = RootfsInstaller(resolver: resolver)
    if let identity = self.deviceMeshIdentity {
      let runtime = IOSRuntimeBridge(resolver: resolver, installer: installer, identity: identity)
      runtime.register(messenger: self.registrar(forPlugin: "IOSRuntimeBridge")!.messenger())
      self.runtimeBridge = runtime
    }
    self.rootfsHandler = RootfsInstallMethodHandler(installer: installer, resolver: resolver)
    self.rootfsHandler?.register(with: self.registrar(forPlugin: "RootfsInstallBridge")!)

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  override func application(
    _ application: UIApplication,
    didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
  ) {
    notificationPlatform?.didRegisterRemoteNotifications(
      deviceToken: deviceToken
    )
    super.application(
      application,
      didRegisterForRemoteNotificationsWithDeviceToken: deviceToken
    )
  }

  override func application(
    _ application: UIApplication,
    didFailToRegisterForRemoteNotificationsWithError error: Error
  ) {
    notificationPlatform?.didFailRemoteNotifications(error: error)
    super.application(
      application,
      didFailToRegisterForRemoteNotificationsWithError: error
    )
  }

  override func application(
    _ application: UIApplication,
    didReceiveRemoteNotification userInfo: [AnyHashable: Any],
    fetchCompletionHandler completionHandler:
      @escaping (UIBackgroundFetchResult) -> Void
  ) {
    if let notificationPlatform,
       notificationPlatform.handleRemoteNotification(
         userInfo,
         completion: { completionHandler(.newData) }
       ) {
      return
    }
    super.application(
      application,
      didReceiveRemoteNotification: userInfo,
      fetchCompletionHandler: completionHandler
    )
  }

  func transportDidBecomeReady(_ transport: IOSNativeTransport) {
    iosNativeHost?.refreshAuthorization()
  }

  func transportDidBecomeUnready(_ transport: IOSNativeTransport) {
  }

  override func applicationDidBecomeActive(_ application: UIApplication) {
    super.applicationDidBecomeActive(application)
    IOSScreenAwakeNativeHandler.apply(application, foreground: true)
  }

  override func applicationWillResignActive(_ application: UIApplication) {
    IOSScreenAwakeNativeHandler.apply(application, foreground: false)
    super.applicationWillResignActive(application)
  }

  override func applicationDidEnterBackground(_ application: UIApplication) {
    runtimeBridge?.suspend()
    IOSScreenAwakeNativeHandler.apply(application, foreground: false)
    IOSSandboxBridge.shared().applicationDidEnterBackground()
    self.iosNativeHost?.didEnterBackground()
    super.applicationDidEnterBackground(application)
  }

  override func applicationWillEnterForeground(_ application: UIApplication) {
    runtimeBridge?.resume()
    IOSSandboxBridge.shared().applicationWillEnterForeground()
    self.iosNativeHost?.willEnterForeground()
    super.applicationWillEnterForeground(application)
  }

  override func applicationWillTerminate(_ application: UIApplication) {
    runtimeBridge?.terminate()
    IOSSandboxBridge.shared().applicationWillTerminate()
    self.iosNativeHost?.willTerminate()
    super.applicationWillTerminate(application)
  }
}
