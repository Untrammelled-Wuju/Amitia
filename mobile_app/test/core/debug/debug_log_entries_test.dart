import 'package:amitia_app/core/debug/debug_log_service.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('首次打开调试日志时返回已有记录', () async {
    final service = DebugLogService();
    service.addRuntimeLog('打开页面之前的启动记录');
    final container = ProviderContainer(overrides: [
      debugLogServiceProvider.overrideWithValue(service),
    ]);
    addTearDown(container.dispose);
    addTearDown(service.dispose);
    final entries = await container.read(debugLogEntriesProvider.future);
    expect(entries.single.message, '打开页面之前的启动记录');
  });
}
