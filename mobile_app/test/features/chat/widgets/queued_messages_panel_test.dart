import 'package:amitia_app/features/chat/presentation/widgets/queued_messages_panel.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('长按拖拽排序并保留引导与删除入口', (tester) async {
    List<String> reordered = <String>[];
    String? steered;
    String? deleted;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 900,
            height: 400,
            child: QueuedMessagesPanel(
              items: const [
                {'requestId': 'a', 'message': '第一条', 'status': 'queued'},
                {'requestId': 'b', 'message': '第二条', 'status': 'queued'},
              ],
              busy: false,
              canSteer: true,
              error: '',
              onReorder: (ids) => reordered = ids,
              onSteer: (id) => steered = id,
              onDelete: (id) => deleted = id,
              onShowError: (_) {},
            ),
          ),
        ),
      ),
    );
    final handles = find.byType(ReorderableDelayedDragStartListener);
    expect(handles, findsNWidgets(2));
    final gesture = await tester.startGesture(tester.getCenter(handles.at(0)));
    await tester.pump(kLongPressTimeout + const Duration(milliseconds: 200));
    await gesture.moveBy(const Offset(0, 120));
    await tester.pump(const Duration(milliseconds: 600));
    await gesture.up();
    await tester.pumpAndSettle();
    expect(reordered, ['b', 'a']);

    await tester.tap(find.widgetWithText(TextButton, '引导').first);
    await tester.tap(find.byTooltip('删除排队消息').last);
    expect(steered, isNotNull);
    expect(deleted, isNotNull);
  });

  testWidgets('失败消息展示原始错误且不可引导', (tester) async {
    String? error;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: QueuedMessagesPanel(
            items: const [
              {
                'requestId': 'failed',
                'message': '失败消息',
                'status': 'uncertain',
                'error': '底层错误\n完整原因链',
              },
            ],
            busy: false,
            canSteer: true,
            error: '',
            onReorder: (_) {},
            onSteer: (_) {},
            onDelete: (_) {},
            onShowError: (value) => error = value,
          ),
        ),
      ),
    );
    expect(find.text('状态不确定'), findsOneWidget);
    final steer = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '引导'),
    );
    expect(steer.onPressed, isNull);
    await tester.tap(find.text('查看原始错误'));
    expect(error, '底层错误\n完整原因链');
  });
}
