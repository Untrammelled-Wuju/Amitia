import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../native_bridge/native_bridge_platform_dispatcher.dart';
import '../native_bridge/providers/native_bridge_relay_provider.dart';

final backgroundKeepAliveServiceProvider = Provider(
  (ref) => BackgroundKeepAliveService(
    ref.read(nativeBridgePlatformDispatcherProvider),
  ),
);

class BackgroundKeepAliveStatus {
  const BackgroundKeepAliveStatus({
    required this.enabled,
    required this.active,
    required this.batteryUnrestricted,
  });
  final bool enabled;
  final bool active;
  final bool batteryUnrestricted;
}

class BackgroundKeepAliveService {
  BackgroundKeepAliveService(this.dispatcher, {String? platform})
    : platform =
          platform ??
          (defaultTargetPlatform == TargetPlatform.android && !kIsWeb
              ? 'android'
              : null);
  final NativeBridgePlatformDispatcher dispatcher;
  final String? platform;

  Future<BackgroundKeepAliveStatus> load() =>
      _execute('device.keep_alive.status', {});
  Future<BackgroundKeepAliveStatus> setEnabled(bool value) async {
    if (value) {
      await _executeRaw('notification.request_permission', {});
    }
    final status = await _execute('device.keep_alive.set', {'enabled': value});
    if (status.enabled != value || value && !status.active) {
      throw StateError('后台保活未成功启动，请检查系统后台限制');
    }
    return status;
  }

  Future<BackgroundKeepAliveStatus> openBatterySettings() =>
      _execute('device.keep_alive.battery_settings', {});

  Future<Map<String, dynamic>> _executeRaw(
    String operation,
    Map<String, dynamic> payload,
  ) async {
    if (platform != 'android') throw UnsupportedError('当前平台不支持后台保活');
    final response = await dispatcher.execute({
      'protocolVersion': 1,
      'requestId': 'keep-alive-${DateTime.now().microsecondsSinceEpoch}',
      'platform': platform,
      'operation': operation,
      'payload': payload,
    });
    if (!['ok', 'success'].contains(response['status'])) {
      final error = response['error'];
      throw StateError(
        error is Map ? error['message']?.toString() ?? '后台保活操作失败' : '后台保活操作失败',
      );
    }
    return response;
  }

  Future<BackgroundKeepAliveStatus> _execute(
    String operation,
    Map<String, dynamic> payload,
  ) async {
    final response = await _executeRaw(operation, payload);
    final result = response['result'];
    if (result is! Map ||
        result['enabled'] is! bool ||
        result['active'] is! bool ||
        result['batteryUnrestricted'] is! bool) {
      throw StateError('后台保活状态无效');
    }
    return BackgroundKeepAliveStatus(
      enabled: result['enabled'] as bool,
      active: result['active'] as bool,
      batteryUnrestricted: result['batteryUnrestricted'] as bool,
    );
  }
}
