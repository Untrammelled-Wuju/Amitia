import 'package:amitia_app/core/debug/debug_log_service.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('首次打开调试日志时返回已有记录', () async {
    final service = DebugLogService();
    service.addRuntimeLog('打开页面之前的启动记录');
    final container = ProviderContainer(
      overrides: [debugLogServiceProvider.overrideWithValue(service)],
    );
    addTearDown(container.dispose);
    addTearDown(service.dispose);
    final entries = await container.read(debugLogEntriesProvider.future);
    expect(entries.single.message, '打开页面之前的启动记录');
  });

  test('错误记录不会被普通调试日志挤出', () {
    final service = DebugLogService();
    addTearDown(service.dispose);
    service.log(DebugLogLevel.error, 'flutter.error', '启动阶段异常');
    for (var index = 0; index < 600; index++) {
      service.log(DebugLogLevel.debug, 'flutter', '普通日志 $index');
    }
    expect(
      service.entries.where((entry) => entry.message == '启动阶段异常'),
      isEmpty,
    );
    expect(service.errorEntries.single.message, '启动阶段异常');
  });
}
