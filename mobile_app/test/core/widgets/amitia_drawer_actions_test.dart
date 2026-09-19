import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('conversation actions use a long press menu', () {
    final source = File(
      'lib/core/widgets/amitia_drawer.dart',
    ).readAsStringSync();

    expect(source, contains('onLongPress: () => _showActions(context)'));
    expect(source, contains('Future<void> _showActions(BuildContext context)'));
    expect(source, contains('showGeneralDialog<_ConversationAction>'));
    expect(source, contains('FadeTransition'));
    expect(source, contains('ScaleTransition'));
    expect(source, contains('alignment: Alignment.bottomRight'));
    expect(source, contains('width: 190'));
    expect(source, contains('BorderRadius.circular(14)'));
    expect(source, contains('offset: Offset.zero'));
    expect(source, contains('blurRadius: 14'));
    expect(source, contains('spreadRadius: 0'));
    expect(source, contains('alignment: Alignment.topLeft'));
    expect(source, contains('const menuWidth = 190.0'));
    expect(source, contains('left: origin.dx + 12 + menuWidth / 2'));
    expect(source, contains('height: 46'));
    expect(source, contains('Divider('));
    expect(source, contains("label: '重命名'"));
    expect(source, contains("label: '归档'"));
    expect(source, contains("label: pinned ? '取消置顶' : '置顶'"));
    expect(source, isNot(contains('showModalBottomSheet<void>')));
    expect(source, isNot(contains('PopupMenuButton<_ConversationAction>')));
    expect(source, isNot(contains('Icons.chat_bubble_outline')));
    expect(source, isNot(contains('Icons.more_horiz')));
    expect(source, contains('fontSize: 14.5'));
    expect(source, contains('minTileHeight: compact ? 34 : 38'));
    expect(source, contains('minVerticalPadding: 0'));
    expect(source, contains('visualDensity: VisualDensity.compact'));
  });
}
