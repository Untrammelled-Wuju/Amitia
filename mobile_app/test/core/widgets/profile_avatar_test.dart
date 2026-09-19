import 'dart:io';

import 'package:amitia_app/core/widgets/profile_avatar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const pngData =
      'data:image/png;base64,'
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';

  testWidgets('profile avatar renders uploaded image', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: ProfileAvatar(avatar: pngData, initial: '我', size: 48),
        ),
      ),
    );

    expect(find.byType(Image), findsOneWidget);
    expect(find.text('我'), findsNothing);
  });

  testWidgets('profile avatar falls back to the user initial', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: ProfileAvatar(avatar: '', initial: '我', size: 48),
        ),
      ),
    );

    expect(find.text('我'), findsOneWidget);
  });

  test('drawer profile row opens settings without avatar upload', () {
    final source = File(
      'lib/core/widgets/amitia_drawer.dart',
    ).readAsStringSync();

    expect(source, contains('onTap: onSettingsTap'));
    expect(source, isNot(contains('onAvatarTap')));
    expect(source, isNot(contains('showEditBadge: true')));
    expect(source, contains('width: 36'));
    expect(source, contains('size: 17'));
    expect(source, contains("number('iconSize', 19)"));
    expect(source, contains('fontSize: 14.5'));
    expect(source, contains("number('outerPaddingY', 0)"));
    expect(source, contains("number('paddingY', 8)"));
  });
}
