import 'package:amitia_app/app/theme/app_theme.dart';
import 'package:amitia_app/core/widgets/amitia_system_bars.dart';
import 'package:amitia_app/features/chat/presentation/widgets/floating_chat_composer_layout.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('navigation bar stays transparent when the theme changes', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1;
    tester.view.padding = const FakeViewPadding(top: 24, bottom: 20);
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPadding);
    final calls = <MethodCall>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        calls.add(call);
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      ),
    );
    final mode = ValueNotifier<ThemeMode>(ThemeMode.light);
    addTearDown(mode.dispose);

    await tester.pumpWidget(
      ValueListenableBuilder<ThemeMode>(
        valueListenable: mode,
        builder: (context, value, child) => MaterialApp(
          theme: AppTheme.lightTheme(),
          darkTheme: AppTheme.darkTheme(),
          themeMode: value,
          home: AmitiaSystemBars(
            child: Scaffold(
              appBar: AppBar(title: const Text('settings')),
              body: const SizedBox.expand(),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    Map<dynamic, dynamic> latestStyle() =>
        calls
                .lastWhere(
                  (call) =>
                      call.method == 'SystemChrome.setSystemUIOverlayStyle',
                )
                .arguments
            as Map<dynamic, dynamic>;

    expect(latestStyle()['systemNavigationBarColor'], 0);
    expect(latestStyle()['systemNavigationBarContrastEnforced'], false);
    expect(
      latestStyle()['systemNavigationBarIconBrightness'],
      'Brightness.dark',
    );
    mode.value = ThemeMode.dark;
    await tester.pumpAndSettle();
    expect(latestStyle()['systemNavigationBarColor'], 0);
    expect(latestStyle()['systemNavigationBarContrastEnforced'], false);
    expect(
      latestStyle()['systemNavigationBarIconBrightness'],
      'Brightness.light',
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('edge to edge keeps composer clear of navigation and keyboard', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1;
    tester.view.padding = const FakeViewPadding(top: 24, bottom: 20);
    tester.view.viewPadding = const FakeViewPadding(top: 24, bottom: 20);
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPadding);
    addTearDown(tester.view.resetViewPadding);
    addTearDown(tester.view.resetViewInsets);

    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.lightTheme(),
        home: AmitiaSystemBars(
          child: Scaffold(
            body: FloatingChatComposerLayout(
              composer: const SafeArea(
                top: false,
                child: SizedBox(key: ValueKey('composer'), height: 100),
              ),
              contentBuilder: (context, height) =>
                  const SafeArea(child: SizedBox(key: ValueKey('messages'))),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      tester.getBottomLeft(find.byKey(const ValueKey('composer'))).dy,
      780,
    );
    expect(
      tester.getBottomLeft(find.byKey(const ValueKey('messages'))).dy,
      780,
    );
    tester.view.viewInsets = const FakeViewPadding(bottom: 300);
    tester.view.padding = const FakeViewPadding(top: 24);
    await tester.pumpAndSettle();
    expect(
      tester.getBottomLeft(find.byKey(const ValueKey('composer'))).dy,
      500,
    );
    expect(
      tester.getBottomLeft(find.byKey(const ValueKey('messages'))).dy,
      500,
    );
    expect(tester.takeException(), isNull);
  });
}
