import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('角色创建后刷新会话和角色目录缓存', () {
    final source = File(
      'lib/features/characters/presentation/pages/character_create_page.dart',
    ).readAsStringSync();

    expect(source, contains('currentCharacterIdProvider.notifier'));
    expect(source, contains('ref.invalidate(characterListProvider)'));
    expect(source, contains('ref.read(chatServiceProvider).owned.refresh()'));
    expect(source, contains('ref.invalidate(conversationListProvider)'));
    expect(source, contains('ref.invalidate(conversationSidebarProvider)'));
  });
}
