import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../device_timezone_cache.dart';
import 'native_bridge_relay_provider.dart';

/// Loads the canonical device IANA timezone from the platform native provider.
/// The value is cached for the synchronous HTTP transport header path.
final deviceTimezoneBootstrapProvider = FutureProvider<String?>((ref) async {
  if (kIsWeb) return null;

  final platform = switch (defaultTargetPlatform) {
    TargetPlatform.android => 'android',
    TargetPlatform.iOS => 'ios',
    TargetPlatform.windows => 'windows',
    _ => null,
  };
  if (platform == null) return null;

  try {
    final dispatcher = ref.watch(nativeBridgePlatformDispatcherProvider);
    final response = await dispatcher.execute(<String, dynamic>{
      'protocolVersion': 1,
      'requestId': 'device-timezone-${DateTime.now().microsecondsSinceEpoch}',
      'platform': platform,
      'operation': 'device.timezone.get',
      'payload': const <String, dynamic>{},
    });
    final status = (response['status'] ?? '').toString().toLowerCase();
    if (status != 'success' && status != 'ok') return null;
    final rawResult = response['result'];
    if (rawResult is! Map) return null;
    final result = Map<String, dynamic>.from(rawResult);
    final timezone = (result['ianaTimezone'] ?? '').toString().trim();
    DeviceTimezoneCache.update(timezone);
    return DeviceTimezoneCache.hasValue ? DeviceTimezoneCache.ianaTimezone : null;
  } catch (_) {
    return null;
  }
});
