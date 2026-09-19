import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('conversation actions use an overflow menu', () {
    final source = File(
      'lib/core/widgets/amitia_drawer.dart',
    ).readAsStringSync();

    expect(source, contains('PopupMenuButton<_ConversationAction>'));
    expect(
      source,
      contains("label: conversation.pinnedAt.isEmpty ? '置顶' : '取消置顶'"),
    );
    expect(source, contains("label: '归档'"));
    expect(source, contains("label: '重命名'"));
    expect(
      source,
      contains('PopupMenuItem(\n            value: _ConversationAction.rename'),
    );
    expect(
      source,
      isNot(
        contains('behavior: HitTestBehavior.opaque,\n        onTap: onRename'),
      ),
    );
    expect(source, contains('fontSize: 14.5'));
    expect(
      source,
      contains('Icons.chat_bubble_outline,\n              size: 19'),
    );
    expect(source, contains('Icon(Icons.more_horiz, size: 19'));
    expect(
      source,
      isNot(contains("tooltip: '归档',\n            onPressed: onArchive")),
    );
  });
}
