import 'package:amitia_app/core/settings/chat_appearance_preferences.dart';
import 'package:amitia_app/features/settings/presentation/widgets/chat_appearance_settings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class TestAppearancePreferences extends ChatAppearancePreferencesNotifier {
  TestAppearancePreferences(this.shouldFail);
  final bool Function() shouldFail;

  @override
  Future<void> init() async {
    if (shouldFail()) throw StateError('外观读取原始错误');
    await super.init();
  }
}

void main() {
  testWidgets('读取失败后禁用开关提供错误详情并支持重试', (tester) async {
    SharedPreferences.setMockInitialValues({});
    var fail = true;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          chatAppearancePreferencesProvider.overrideWith(
            (ref) => TestAppearancePreferences(() => fail),
          ),
        ],
        child: const MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(child: ChatAppearanceSettings()),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final target = find.byType(SwitchListTile).first;
    expect(tester.widget<SwitchListTile>(target).onChanged, isNull);
    await tester.tap(target);
    await tester.pumpAndSettle();
    expect(find.textContaining('外观读取原始错误'), findsOneWidget);
    fail = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(tester.widget<SwitchListTile>(target).onChanged, isNotNull);
  });
}
