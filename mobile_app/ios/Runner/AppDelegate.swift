import Flutter
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate {
  private var sandboxHandler: IOSSandboxMethodHandler?
  private var rootfsHandler: RootfsInstallMethodHandler?

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    GeneratedPluginRegistrant.register(with: self)

    let bridge = IOSSandboxBridge.shared()
    self.sandboxHandler = IOSSandboxMethodHandler(bridge: bridge)
    self.sandboxHandler?.register(with: self.registrar(forPlugin: "IOSSandboxBridge")!)

    let resolver = RootfsResolver()
    let installer = RootfsInstaller(resolver: resolver)
    self.rootfsHandler = RootfsInstallMethodHandler(installer: installer, resolver: resolver)
    self.rootfsHandler?.register(with: self.registrar(forPlugin: "RootfsInstallBridge")!)

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }
}
