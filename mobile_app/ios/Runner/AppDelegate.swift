import Flutter
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate, IOSNativeTransportDelegate {
  private var sandboxHandler: IOSSandboxMethodHandler?
  private var rootfsHandler: RootfsInstallMethodHandler?
  private var iosNativeHost: IOSNativeHost?
  private var nativeTransport: IOSNativeTransport?

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    GeneratedPluginRegistrant.register(with: self)

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

    if let host = self.iosNativeHost {
      self.nativeTransport = IOSNativeTransport(host: host, delegate: self)
      self.nativeTransport?.attach()
      let nativeRegistrar = self.registrar(forPlugin: "IOSNativeBridgePlugin")!
      let nativeMessenger = nativeRegistrar.messenger()
      AudioRecorderAdapter.shared.registerRealtimeChannels(messenger: nativeMessenger)
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
    self.rootfsHandler = RootfsInstallMethodHandler(installer: installer, resolver: resolver)
    self.rootfsHandler?.register(with: self.registrar(forPlugin: "RootfsInstallBridge")!)

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  func transportDidBecomeReady(_ transport: IOSNativeTransport) {
    iosNativeHost?.refreshAuthorization()
  }

  func transportDidBecomeUnready(_ transport: IOSNativeTransport) {
  }

  override func applicationDidEnterBackground(_ application: UIApplication) {
    IOSSandboxBridge.shared().applicationDidEnterBackground()
    self.iosNativeHost?.didEnterBackground()
    super.applicationDidEnterBackground(application)
  }

  override func applicationWillEnterForeground(_ application: UIApplication) {
    IOSSandboxBridge.shared().applicationWillEnterForeground()
    self.iosNativeHost?.willEnterForeground()
    super.applicationWillEnterForeground(application)
  }

  override func applicationWillTerminate(_ application: UIApplication) {
    IOSSandboxBridge.shared().applicationWillTerminate()
    self.iosNativeHost?.willTerminate()
    super.applicationWillTerminate(application)
  }
}
