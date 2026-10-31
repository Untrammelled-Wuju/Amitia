import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../app_update/app_update_providers.dart';
import '../debug/debug_log_service.dart';
import '../runtime/backend/mobile_backend_providers.dart';
import '../runtime/backend/mobile_deployment_mode.dart';
import '../ui_runtime/ui_client_info.dart';
import '../ui_runtime/ui_device_identity.dart';

Future<String> _resolveAppVersion(WidgetRef ref) async {
  try {
    final installed = await ref
        .read(appUpdateServiceProvider)
        .getInstalledInfo();
    if (installed.versionName.trim().isNotEmpty) {
      return installed.versionName.trim();
    }
  } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
}
  return currentUIClientInfo().appVersion;
}

String _resolveRuntimeMode(WidgetRef ref) {
  try {
    final config = ref.read(mobileDeploymentConfigProvider);
    return config.mode == MobileDeploymentMode.cloud ? 'cloud' : 'local';
  } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
return 'local';
  }
}

Future<String> _resolveDeviceId() async {
  try {
    return await UIDeviceIdentity().getOrCreate();
  } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
return '';
  }
}

List<Map<String, dynamic>> collectUserFeedbackClientErrors(WidgetRef ref) {
  final entries = ref
      .read(debugLogServiceProvider)
      .errorEntries
      .toList(growable: false);
  final recent = entries.length > 15
      ? entries.sublist(entries.length - 15)
      : entries;
  return recent
      .map(
        (entry) => <String, dynamic>{
          'message': entry.message,
          'stack': entry.stackTrace ?? '',
          'source': entry.source,
          'at': entry.time.toIso8601String(),
        },
      )
      .toList(growable: false);
}

String userFeedbackTimezone() {
  final now = DateTime.now();
  final offset = now.timeZoneOffset;
  final sign = offset.isNegative ? '-' : '+';
  final hours = offset.inHours.abs().toString().padLeft(2, '0');
  final minutes = (offset.inMinutes.abs() % 60).toString().padLeft(2, '0');
  return '${now.timeZoneName} UTC$sign$hours:$minutes';
}

String userFeedbackMediaTypeForPath(String path) {
  final lower = path.toLowerCase();
  if (lower.endsWith('.jpg') || lower.endsWith('.jpeg')) return 'image/jpeg';
  if (lower.endsWith('.webp')) return 'image/webp';
  if (lower.endsWith('.gif')) return 'image/gif';
  return 'image/png';
}

Future<Map<String, dynamic>> buildUserFeedbackPayload({
  required WidgetRef ref,
  required String category,
  required String title,
  required String description,
  required String contact,
  required bool includeLogs,
  required String uploadUrl,
  Map<String, dynamic>? screenshot,
}) async {
  final appVersion = await _resolveAppVersion(ref);
  final runtimeMode = _resolveRuntimeMode(ref);
  final deviceId = await _resolveDeviceId();
  return <String, dynamic>{
    'category': category,
    'title': title,
    'description': description,
    'contact': contact,
    'includeLogs': includeLogs,
    'clientErrors': collectUserFeedbackClientErrors(ref),
    'platform': Platform.isAndroid
        ? 'android'
        : Platform.isIOS
        ? 'ios'
        : Platform.operatingSystem,
    'clientType': 'mobile',
    'appVersion': appVersion,
    'runtimeMode': runtimeMode,
    'deviceId': deviceId,
    'osInfo': '${Platform.operatingSystem} ${Platform.operatingSystemVersion}',
    'locale': Platform.localeName,
    'timezone': userFeedbackTimezone(),
    'uploadUrl': uploadUrl,
    'screenshot': ?screenshot,
  };
}
