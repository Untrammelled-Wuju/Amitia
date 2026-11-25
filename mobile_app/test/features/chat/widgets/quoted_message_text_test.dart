import 'package:amitia_app/core/widgets/quoted_message_text.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('长引用可展开查看完整原文并收起', (tester) async {
    final content = '原始消息内容' * 30;
    await tester.pumpWidget(MaterialApp(home: Scaffold(body: QuotedMessageText(text: content))));
    expect(tester.widget<Text>(find.text(content)).maxLines, 2);
    await tester.tap(find.text('展开'));
    await tester.pump();
    expect(tester.widget<Text>(find.text(content)).maxLines, isNull);
    await tester.tap(find.text('收起'));
    await tester.pump();
    expect(tester.widget<Text>(find.text(content)).maxLines, 2);
  });
}
