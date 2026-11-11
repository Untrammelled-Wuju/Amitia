import 'package:amitia_app/core/settings/chat_input_preferences.dart';
import 'package:amitia_app/features/settings/presentation/widgets/chat_input_settings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class FailingPreferences extends ChatInputPreferencesNotifier {
  @override
  Future<void> init() async => throw StateError('读取失败原文');
}

void main() {
  testWidgets('读取失败后点击开关显示原始错误并可重试', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          chatInputPreferencesProvider.overrideWith(
            (ref) => FailingPreferences(),
          ),
        ],
        child: const MaterialApp(home: Scaffold(body: ChatInputSettings())),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      tester.widget<SwitchListTile>(find.byType(SwitchListTile)).onChanged,
      isNull,
    );
    await tester.tap(find.byType(SwitchListTile));
    await tester.pumpAndSettle();
    expect(find.textContaining('读取失败原文'), findsOneWidget);
    expect(find.text('重试'), findsOneWidget);
    expect(tester.widget<Switch>(find.byType(Switch)).value, false);
  });
}
