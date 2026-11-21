import Flutter
import UIKit

final class IOSSystemDeviceName: NSObject {
  static let channelName = "com.amitia.system/device"

  private var channel: FlutterMethodChannel?

  func register(messenger: FlutterBinaryMessenger) {
    let channel = FlutterMethodChannel(
      name: Self.channelName,
      binaryMessenger: messenger
    )
    channel.setMethodCallHandler { call, result in
      switch call.method {
      case "deviceName":
        result(Self.deviceName())
      default:
        result(FlutterMethodNotImplemented)
      }
    }
    self.channel = channel
  }

  static func deviceName() -> String {
    let assigned = UIDevice.current.name
      .trimmingCharacters(in: .whitespacesAndNewlines)
    if !assigned.isEmpty && assigned != "iPhone" && assigned != "iPad" {
      return assigned
    }
    return UIDevice.current.model
  }
}
