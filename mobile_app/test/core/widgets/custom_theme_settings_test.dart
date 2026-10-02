import 'package:amitia_app/core/settings/appearance_preferences.dart';
import 'package:amitia_app/core/settings/custom_palette.dart';
import 'package:amitia_app/features/settings/presentation/widgets/custom_theme_settings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('color editor validates hex and applies a custom text color', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final container = ProviderContainer();
    addTearDown(container.dispose);
    await container.read(appearancePreferencesProvider.notifier).init();
    await container.read(appearancePreferencesProvider.notifier).setCustomPalette(const CustomPalette(enabled: true));
    await tester.pumpWidget(UncontrolledProviderScope(container: container, child: const MaterialApp(home: Scaffold(body: SingleChildScrollView(child: CustomThemeSettings())))));
    await tester.tap(find.text('文字颜色'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '#xyz');
    await tester.pump();
    expect(find.text('请输入六位十六进制颜色'), findsOneWidget);
    await tester.enterText(find.byType(TextField), '#123456');
    await tester.pump();
    await tester.tap(find.text('应用'));
    await tester.pumpAndSettle();
    expect(container.read(appearancePreferencesProvider).customPalette.text, const Color(0xFF123456));
    expect(container.read(appearancePreferencesProvider).customPalette.textMode, ThemeTextMode.custom);
    expect(tester.takeException(), isNull);
  });
}
