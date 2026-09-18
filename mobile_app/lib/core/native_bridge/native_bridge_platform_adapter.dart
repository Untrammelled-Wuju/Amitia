import 'dart:io' show Platform;
import 'package:flutter/foundation.dart';
import '../backend_connection/backend_connection_config.dart';
import 'native_bridge_platform_dispatcher.dart';
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
    required BackendConnectionConfig connectionConfig,
    required NativeBridgePlatformDispatcher dispatcher,
    String? platform,
    Duration reconnectDelay = const Duration(seconds: 3),
  }) {
    return NativeBridgeRelayClient(
      connectionConfig: connectionConfig,
      dispatcher: dispatcher,
      platform: platform ?? NativeBridgePlatformAdapter.platform,
      reconnectDelay: reconnectDelay,
    );
  }
}
