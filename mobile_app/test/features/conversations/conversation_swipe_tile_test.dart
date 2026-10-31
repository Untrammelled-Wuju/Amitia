import 'package:amitia_app/features/conversations/presentation/widgets/conversation_swipe_tile.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('向左滑动后可以置顶、归档并标为已读', (tester) async {
    var pinned = 0;
    var archived = 0;
    var read = 0;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 800,
            child: ConversationSwipeTile(
              key: const ValueKey('swipe-tile'),
              pinned: false,
              readable: true,
              onPin: () async => pinned++,
              onArchive: () async => archived++,
              onMarkRead: () async => read++,
              child: const SizedBox(height: 76, child: Text('测试消息')),
            ),
          ),
        ),
      ),
    );

    await tester.drag(find.text('测试消息'), const Offset(-240, 0));
    await tester.pumpAndSettle();
    expect(find.text('置顶'), findsOneWidget);
    expect(find.text('归档'), findsOneWidget);
    expect(find.text('标为已读'), findsOneWidget);

    await tester.tap(find.text('置顶'));
    await tester.pumpAndSettle();
    await tester.drag(find.text('测试消息'), const Offset(-240, 0));
    await tester.pumpAndSettle();
    await tester.tap(find.text('归档'));
    await tester.pumpAndSettle();
    await tester.drag(find.text('测试消息'), const Offset(-240, 0));
    await tester.pumpAndSettle();
    await tester.tap(find.text('标为已读'));
    await tester.pumpAndSettle();

    expect([pinned, archived, read], [1, 1, 1]);
  });
}
