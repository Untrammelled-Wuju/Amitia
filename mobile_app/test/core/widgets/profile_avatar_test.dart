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
}
