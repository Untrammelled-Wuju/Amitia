import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('chat call icon matches adjacent top bar icon sizing', () {
    final source = File(
      'lib/features/chat/presentation/pages/chat_page.dart',
    ).readAsStringSync();

    expect(source, contains('icon: Icons.phone_in_talk_outlined'));
    expect(source, contains('iconSize: 20'));
    expect(source, isNot(contains('icon: Icons.call_outlined')));
    expect(source, contains('menuWidth: 190'));
    expect(source, contains('itemFontSize: 14.5'));
    expect(source, contains('itemIconSize: 18'));
    expect(source, contains('itemHorizontalPadding: 14'));
    expect(source, contains('itemMinHeight: 46'));
    expect(source, isNot(contains('PopupMenuDivider')));
  });
}
