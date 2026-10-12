import 'package:amitia_app/features/chat/presentation/widgets/floating_chat_composer_layout.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('messages fill the safe area behind the floating composer', (
    tester,
  ) async {
    const viewport = Size(400, 800);
    const safeInsets = EdgeInsets.only(top: 24, bottom: 20);
    tester.view.physicalSize = viewport;
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final controller = ScrollController();
    addTearDown(controller.dispose);
    var taps = 0;

    await tester.pumpWidget(
      MaterialApp(
        home: MediaQuery(
          data: const MediaQueryData(size: viewport, padding: safeInsets),
          child: Scaffold(
            body: FloatingChatComposerLayout(
              composer: const SafeArea(
                top: false,
                child: SizedBox(height: 120),
              ),
              contentBuilder: (context, composerHeight) => SafeArea(
                child: ListView.builder(
                  key: const ValueKey('messages'),
                  controller: controller,
                  itemExtent: 60,
                  itemCount: 20,
                  padding: EdgeInsets.only(
                    bottom: (composerHeight - safeInsets.bottom).clamp(
                      0.0,
                      double.infinity,
                    ),
                  ),
                  itemBuilder: (context, index) => GestureDetector(
                    key: index == 19 ? const ValueKey('last-message') : null,
                    onTap: () => taps++,
                    child: Text(index == 0 ? 'first' : 'message $index'),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(tester.getTopLeft(find.byKey(const ValueKey('messages'))).dy, 24);
    expect(
      tester.getBottomLeft(find.byKey(const ValueKey('messages'))).dy,
      780,
    );
    await tester.tap(find.text('first'));
    expect(taps, 1);
    controller.jumpTo(controller.position.maxScrollExtent);
    await tester.pump();
    expect(
      tester.getBottomLeft(find.byKey(const ValueKey('last-message'))).dy,
      660,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'composer growth and keyboard resize preserve the full viewport',
    (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final height = ValueNotifier<double>(110);
      addTearDown(height.dispose);
      var composerHeight = 0.0;
      var changes = 0;
      Widget buildPage(double viewportHeight) => MaterialApp(
        home: Align(
          alignment: Alignment.topLeft,
          child: SizedBox(
            width: 400,
            height: viewportHeight,
            child: FloatingChatComposerLayout(
              onComposerHeightChanged: () => changes++,
              composer: ValueListenableBuilder<double>(
                valueListenable: height,
                builder: (context, value, child) =>
                    SizedBox(key: const ValueKey('composer'), height: value),
              ),
              contentBuilder: (context, value) {
                composerHeight = value;
                return const SizedBox(key: ValueKey('viewport'));
              },
            ),
          ),
        ),
      );

      await tester.pumpWidget(buildPage(700));
      await tester.pump();
      expect(composerHeight, 110);
      height.value = 190;
      await tester.pump();
      await tester.pump();
      expect(composerHeight, 190);
      expect(
        tester.getSize(find.byKey(const ValueKey('viewport'))).height,
        700,
      );
      expect(tester.getTopLeft(find.byKey(const ValueKey('composer'))).dy, 510);
      expect(changes, 2);

      await tester.pumpWidget(buildPage(400));
      await tester.pump();
      expect(
        tester.getSize(find.byKey(const ValueKey('viewport'))).height,
        400,
      );
      expect(tester.getTopLeft(find.byKey(const ValueKey('composer'))).dy, 210);
      expect(composerHeight, 190);
      expect(changes, 2);
      await tester.pumpWidget(const SizedBox.shrink());
      expect(tester.takeException(), isNull);
    },
  );
}
