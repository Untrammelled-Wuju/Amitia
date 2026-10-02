import 'package:amitia_app/core/widgets/amitia_message.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('soft keyboard sends only when enabled', (tester) async {
    final messages = <String>[];
    for (final enabled in [false, true]) {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: AmitiaChatInput(sendOnEnter: enabled, onSend: messages.add),
          ),
        ),
      );
      await tester.enterText(find.byType(TextField), 'hello');
      expect(
        tester.widget<TextField>(find.byType(TextField)).textInputAction,
        enabled ? TextInputAction.send : TextInputAction.newline,
      );
      await tester.testTextInput.receiveAction(
        enabled ? TextInputAction.send : TextInputAction.newline,
      );
      await tester.pump();
      expect(messages.length, enabled ? 1 : 0);
    }
  });
  testWidgets('hardware enter respects shift and composing text', (
    tester,
  ) async {
    final messages = <String>[];
    final controller = TextEditingController(text: 'hello');
    addTearDown(controller.dispose);
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: AmitiaChatInput(
            sendOnEnter: true,
            controller: controller,
            onSend: messages.add,
          ),
        ),
      ),
    );
    await tester.tap(find.byType(TextField));
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
    expect(messages, isEmpty);
    controller.value = const TextEditingValue(
      text: 'hello',
      composing: TextRange(start: 0, end: 5),
    );
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(messages, isEmpty);
    controller.value = const TextEditingValue(text: 'hello');
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(messages, ['hello']);
  });
}
