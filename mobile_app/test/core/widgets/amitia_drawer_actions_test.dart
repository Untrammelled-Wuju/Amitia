import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('conversation actions use a long press menu', () {
    final source = File(
      'lib/core/widgets/amitia_drawer.dart',
    ).readAsStringSync();

    expect(source, contains('onLongPress: () => _showActions(context)'));
    expect(source, contains('void _showActions(BuildContext context)'));
    expect(source, contains("title: const Text('重命名')"));
    expect(source, contains("title: const Text('归档')"));
    expect(
      source,
      contains("title: Text(conversation.pinnedAt.isEmpty ? '置顶' : '取消置顶')"),
    );
    expect(source, isNot(contains('PopupMenuButton<_ConversationAction>')));
    expect(source, isNot(contains('Icons.chat_bubble_outline')));
    expect(source, isNot(contains('Icons.more_horiz')));
    expect(source, contains('fontSize: 14.5'));
  });
}
