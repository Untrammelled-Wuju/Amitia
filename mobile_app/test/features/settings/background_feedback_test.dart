import 'dart:io';

import 'package:amitia_app/core/settings/background_preferences.dart';
import 'package:amitia_app/features/settings/presentation/widgets/background_settings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  late Directory directory;
  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    directory = await Directory.systemTemp.createTemp('background-feedback-');
  });
  tearDown(() async => directory.delete(recursive: true));
  testWidgets('未选择背景时提示原因并保持控件不可修改', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          backgroundPreferencesProvider.overrideWith(
            (ref) =>
                BackgroundPreferencesNotifier(directory: () async => directory),
          ),
        ],
        child: const MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(child: BackgroundSettings()),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final switches = find.byType(SwitchListTile);
    expect(tester.widget<SwitchListTile>(switches.first).onChanged, isNull);
    await tester.tap(switches.first);
    await tester.pump();
    expect(find.text('请先选择背景。'), findsOneWidget);
    expect(tester.widget<SwitchListTile>(switches.first).value, isFalse);
    ScaffoldMessenger.of(tester.element(switches.first)).hideCurrentSnackBar();
    await tester.pumpAndSettle();
    expect(find.text('请先选择背景。'), findsNothing);
    await tester.tapAt(
      tester.getCenter(find.byType(Slider).first, warnIfMissed: false),
    );
    await tester.pump();
    expect(find.text('请先选择背景。'), findsOneWidget);
  });
}
