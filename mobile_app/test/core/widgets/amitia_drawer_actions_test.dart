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
    expect(
      source,
      isNot(contains("tooltip: '归档',\n            onPressed: onArchive")),
    );
  });
}
