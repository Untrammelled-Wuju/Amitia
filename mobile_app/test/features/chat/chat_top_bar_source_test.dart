import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('chat controls use the shared compact popup menu', () {
    final source = File(
      'lib/features/chat/presentation/pages/chat_page.dart',
    ).readAsStringSync();

    expect(source, contains('icon: Icons.phone_in_talk_outlined'));
    expect(source, contains('AmitiaPopupMenuButton<String>('));
    expect(source, isNot(matches(RegExp(r'\bPopupMenuButton\s*<'))));
    expect(source, isNot(contains('icon: Icons.call_outlined')));
    expect(source, contains('menuWidth: 220'));
    expect(source, contains('itemFontSize: 14.5'));
    expect(source, contains('itemIconSize: 18'));
    expect(source, contains('itemHorizontalPadding: 14'));
    expect(source, contains('itemMinHeight: 46'));
  });
}
