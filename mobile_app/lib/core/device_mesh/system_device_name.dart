import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

const int maxDeviceNameRunes = 64;

const MethodChannel _systemDeviceChannel = MethodChannel(
  'com.amitia.system/device',
);

Future<String> resolveSystemDeviceName() async {
  try {
    final raw = await _systemDeviceChannel.invokeMethod<String>('deviceName');
    final normalized = normalizeDeviceName(raw ?? '');
    if (normalized.isNotEmpty) return normalized;
  } catch (error, stackTrace) {
    print('[original-error] $error\n$stackTrace');
  }
  return normalizeDeviceName(fallbackDeviceName());
}

String fallbackDeviceName() {
  switch (defaultTargetPlatform) {
    case TargetPlatform.android:
      return 'Android 设备';
    case TargetPlatform.iOS:
      return 'iPhone';
    case TargetPlatform.macOS:
      return 'MacBook';
    case TargetPlatform.windows:
      return 'Windows 设备';
    case TargetPlatform.linux:
      return 'Linux 设备';
    case TargetPlatform.fuchsia:
      return 'Fuchsia 设备';
  }
}

String normalizeDeviceName(String raw) {
  final stripped = raw
      .replaceAll(RegExp('[\u0000-\u001F\u007F\u200B\uFEFF]'), '')
      .replaceAll(RegExp(r'\s+'), ' ')
      .trim();
  if (stripped.isEmpty) return '';
  final runes = stripped.runes.toList(growable: false);
  if (runes.length <= maxDeviceNameRunes) return stripped;
  return String.fromCharCodes(runes.take(maxDeviceNameRunes)).trim();
}
