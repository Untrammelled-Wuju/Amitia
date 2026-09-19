import 'dart:io';

import 'package:amitia_app/core/widgets/amitia_message.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('voice composer keeps text vertically aligned with input text', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: AmitiaChatInput(onSend: (_) {})),
      ),
    );

    final textFieldCenter = tester.getCenter(find.text('发消息…')).dy;

    await tester.tap(find.byTooltip('按住说话'));
    await tester.pumpAndSettle();

    final voiceTextCenter = tester.getCenter(find.text('按住说话')).dy;
    final composerTop = tester
        .getTopLeft(find.byKey(const ValueKey('chat-composer-surface')))
        .dy;
    final voiceButtonTop = tester
        .getTopLeft(find.byKey(const ValueKey('hold-to-talk-button')))
        .dy;
    final voiceButtonHeight = tester
        .getSize(find.byKey(const ValueKey('hold-to-talk-button')))
        .height;

    expect(voiceTextCenter, closeTo(textFieldCenter, 0.5));
    expect(voiceButtonTop - composerTop, greaterThanOrEqualTo(6));
    expect(voiceButtonHeight, closeTo(44, 0.1));
  });

  testWidgets('composer border does not change when input is focused', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: AmitiaChatInput(onSend: (_) {})),
      ),
    );

    BoxDecoration decoration() =>
        tester
                .widget<Container>(
                  find.byKey(const ValueKey('chat-composer-surface')),
                )
                .decoration!
            as BoxDecoration;

    final before = decoration().border;
    await tester.tap(find.byType(TextField));
    await tester.pump();
    final after = decoration().border;

    expect(after, before);
  });

  testWidgets('composer trailing action matches the add action inset', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: AmitiaChatInput(onSend: (_) {})),
      ),
    );

    final surface = tester.getRect(
      find.byKey(const ValueKey('chat-composer-surface')),
    );
    final add = tester.getRect(find.byIcon(Icons.add_rounded));
    final voice = tester.getRect(find.byIcon(Icons.mic_none_outlined));

    final addInset = add.left - surface.left;
    final voiceInset = surface.right - voice.right;

    expect(addInset - voiceInset, closeTo(3, 0.5));
    expect(voiceInset, lessThan(addInset));
  });

  test('composer keeps a lower bottom inset when the keyboard is closed', () {
    final source = File(
      'lib/core/widgets/amitia_message.dart',
    ).readAsStringSync();

    expect(
      source,
      contains('padding: const EdgeInsets.fromLTRB(10, 8, 10, 4)'),
    );
  });

  testWidgets('composer text and hold-to-talk use the compact size', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: AmitiaChatInput(onSend: (_) {})),
      ),
    );

    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.style?.fontSize, 15);
    expect(field.decoration?.hintStyle?.fontSize, 15);

    await tester.tap(find.byTooltip('按住说话'));
    await tester.pumpAndSettle();

    final holdToTalk = tester.widget<Text>(find.text('按住说话'));
    expect(holdToTalk.style?.fontSize, 15);
  });
}
