import 'package:amitia_app/features/chat/presentation/widgets/realtime_call_layout.dart';
import 'package:amitia_app/features/conversation/rendering/mermaid/amitia_mermaid_block.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('large call text scrolls while hangup remains accessible', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 480);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    var ended = false;
    final error = List.filled(20, '连接失败，请检查网络与服务状态。').join();
    await tester.pumpWidget(
      MaterialApp(
        home: MediaQuery(
          data: const MediaQueryData(
            size: Size(320, 480),
            textScaler: TextScaler.linear(2),
          ),
          child: Scaffold(
            body: RealtimeCallLayout(
              background: const ColoredBox(color: Colors.black),
              details: Column(
                children: [
                  const Text('很长的角色名称与通话状态', style: TextStyle(fontSize: 21)),
                  RealtimeCallErrorDetails(message: error),
                ],
              ),
              controls: ElevatedButton(
                onPressed: () => ended = true,
                child: const Text('挂断'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('查看完整错误'));
    await tester.pumpAndSettle();
    expect(find.text(error), findsOneWidget);
    expect(tester.takeException(), isNull);
    final hangup = tester.getRect(find.widgetWithText(ElevatedButton, '挂断'));
    expect(hangup.bottom, lessThanOrEqualTo(480));
    await tester.tap(find.text('挂断'));
    expect(ended, isTrue);
  });

  testWidgets('diagram toolbar remains usable with enlarged text', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(
      const MaterialApp(
        home: MediaQuery(
          data: MediaQueryData(
            size: Size(320, 640),
            textScaler: TextScaler.linear(2),
          ),
          child: Scaffold(
            body: AmitiaMermaidBlock(
              source: 'graph TD; A-->B',
              streaming: true,
            ),
          ),
        ),
      ),
    );
    expect(tester.takeException(), isNull);
    for (final button in find.byType(IconButton).evaluate()) {
      final size = tester.getSize(find.byWidget(button.widget));
      expect(size.width, greaterThanOrEqualTo(48));
      expect(size.height, greaterThanOrEqualTo(48));
    }
    await tester.tap(find.byTooltip('源码'));
    await tester.pump();
    expect(find.byTooltip('预览'), findsOneWidget);
  });
}
