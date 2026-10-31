import 'package:amitia_app/core/services/providers.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('返回退出只在两秒窗口内生效', () {
    final handler = ConversationBackHandler();
    final startedAt = DateTime(2026, 10, 31, 22, 0);
    const window = Duration(seconds: 2);

    expect(handler.shouldExit(startedAt, window), isFalse);
    expect(
      handler.shouldExit(startedAt.add(const Duration(milliseconds: 1500)), window),
      isTrue,
    );
    expect(handler.shouldExit(startedAt.add(const Duration(seconds: 4)), window), isFalse);
    expect(
      handler.shouldExit(startedAt.add(const Duration(seconds: 5)), window),
      isTrue,
    );
  });

  test('关闭多选会重置退出窗口', () {
    final handler = ConversationBackHandler();
    final startedAt = DateTime(2026, 10, 31, 22, 0);
    const window = Duration(seconds: 2);

    expect(handler.shouldExit(startedAt, window), isFalse);
    handler.resetExitWindow();
    expect(
      handler.shouldExit(startedAt.add(const Duration(milliseconds: 500)), window),
      isFalse,
    );
  });
}
