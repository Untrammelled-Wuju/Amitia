import 'package:amitia_app/features/chat/presentation/widgets/chat_error_notice.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('错误原始信息完整显示在两个减号中间', (tester) async {
    const detail =
        'errorCode: task_execution_failed\n\n'
        'cause: tool execution cancelled\n\n'
        'rawError: tool execution cancelled';
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: ChatErrorNotice(detail: detail)),
      ),
    );

    expect(find.text('-$detail-'), findsOneWidget);
    expect(find.text('-错误-'), findsNothing);
    expect(find.text('重试'), findsNothing);
  });
}
