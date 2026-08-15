import 'dart:io' show Platform;
import 'package:flutter/foundation.dart';
import 'native_bridge_relay_client.dart';

class NativeBridgePlatformAdapter {
  static String get platform {
    if (kIsWeb) return 'web';
    if (Platform.isIOS) return 'ios';
    if (Platform.isAndroid) return 'android';
    return 'unknown';
  }

  static bool get supportsNativeRelay {
    return platform == 'ios' || platform == 'android';
  }

  static NativeBridgeRelayClient createClient({
    required String baseUrl,
    String? platform,
    Duration heartbeatInterval = const Duration(seconds: 30),
    Duration reconnectDelay = const Duration(seconds: 3),
  }) {
    return NativeBridgeRelayClient(
      baseUrl: baseUrl,
      platform: platform ?? NativeBridgePlatformAdapter.platform,
      heartbeatInterval: heartbeatInterval,
      reconnectDelay: reconnectDelay,
    );
  }
}
