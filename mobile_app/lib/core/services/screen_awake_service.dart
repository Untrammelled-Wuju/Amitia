import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../native_bridge/native_bridge_platform_dispatcher.dart';
import '../native_bridge/providers/native_bridge_relay_provider.dart';

final screenAwakeServiceProvider = Provider(
  (ref) => ScreenAwakeService(ref.read(nativeBridgePlatformDispatcherProvider)),
);

class ScreenAwakeService {
  ScreenAwakeService(this.dispatcher, {String? platform})
    : platform =
          platform ??
          switch (defaultTargetPlatform) {
            TargetPlatform.android => 'android',
            TargetPlatform.iOS => 'ios',
            _ => null,
          };
  final NativeBridgePlatformDispatcher dispatcher;
  final String? platform;

  Future<bool> load() => _execute('display.keep_awake.status', {});
  Future<bool> setEnabled(bool enabled) async {
    final saved = await _execute('display.keep_awake.set', {
      'enabled': enabled,
    });
    if (saved != enabled) throw StateError('屏幕常亮设置未生效');
    return saved;
  }

  Future<bool> _execute(String operation, Map<String, dynamic> payload) async {
    if (platform == null) throw UnsupportedError('当前平台不支持屏幕常亮');
    final response = await dispatcher.execute({
      'protocolVersion': 1,
      'requestId': 'screen-awake-${DateTime.now().microsecondsSinceEpoch}',
      'platform': platform,
      'operation': operation,
      'payload': payload,
    });
    final result = response['result'];
    if (!['ok', 'success'].contains(response['status']) ||
        result is! Map ||
        result['enabled'] is! bool) {
      throw StateError('无法读取或保存屏幕常亮设置');
    }
    return result['enabled'] as bool;
  }
}
