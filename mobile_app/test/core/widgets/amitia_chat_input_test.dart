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
}
