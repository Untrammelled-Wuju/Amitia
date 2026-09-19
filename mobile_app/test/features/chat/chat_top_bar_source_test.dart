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
  });
}
