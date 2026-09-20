import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('chat call control uses the compact size', () {
    final source = File(
      'lib/features/chat/presentation/pages/chat_page.dart',
    ).readAsStringSync();

    expect(source, contains('icon: Icons.phone_in_talk_outlined'));
    expect(
      source,
      contains(
        'icon: Icons.phone_in_talk_outlined,\n                size: 36,\n                iconSize: 20,',
      ),
    );
    expect(
      source,
      contains(
        'icon: isApplePlatform\n'
        '                    ? CupertinoIcons.ellipsis\n'
        '                    : Icons.more_horiz,\n'
        '                size: 44,\n'
        '                iconSize: 20,',
      ),
    );
    expect(source, isNot(contains('icon: Icons.call_outlined')));
    expect(source, contains('menuWidth: 190'));
    expect(source, contains('itemFontSize: 14.5'));
    expect(source, contains('itemIconSize: 18'));
    expect(source, contains('itemHorizontalPadding: 14'));
    expect(source, contains('itemMinHeight: 46'));
    expect(source, isNot(contains('PopupMenuDivider')));
  });
}
