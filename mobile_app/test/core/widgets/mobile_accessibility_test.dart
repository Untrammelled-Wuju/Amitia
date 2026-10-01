import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/app/theme/app_text_scaler.dart';
import 'package:amitia_app/app/theme/app_theme.dart';
import 'package:amitia_app/core/widgets/amitia_button.dart';
import 'package:amitia_app/core/widgets/amitia_misc.dart';
import 'package:amitia_app/core/widgets/amitia_visual_scope.dart';

void main() {
  test('large text is preserved outside the composer', () {
    const scaler = AppTextScaler(TextScaler.linear(3), 1.2);
    expect(scaler.scale(16), closeTo(57.6, 0.001));
    expect(scaler.composer.scale(16), 32);
  });

  testWidgets('icon buttons retain composer sizing', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              AmitiaIconButton(icon: Icons.add, onPressed: () {}),
              AmitiaVisualScope(
                preserveComposer: true,
                child: AmitiaIconButton(icon: Icons.remove, onPressed: () {}),
              ),
            ],
          ),
        ),
      ),
    );
    expect(tester.getSize(find.byType(AmitiaIconButton).first).height, 48);
    expect(tester.getSize(find.byType(AmitiaIconButton).last).height, 40);
  });

  testWidgets('long segments remain selectable at large text sizes', (
    tester,
  ) async {
    var selected = -1;
    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.lightTheme(),
        home: MediaQuery(
          data: const MediaQueryData(textScaler: TextScaler.linear(2)),
          child: Scaffold(
            body: SizedBox(
              width: 320,
              child: AmitiaSegmentedControl(
                segments: const ['基本配置', '很长的高级模型参数设置', '运行状态'],
                selectedIndex: 0,
                onChanged: (value) => selected = value,
              ),
            ),
          ),
        ),
      ),
    );
    expect(tester.takeException(), isNull);
    await tester.drag(
      find.byType(SingleChildScrollView),
      const Offset(-1000, 0),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('运行状态'));
    await tester.pumpAndSettle();
    expect(selected, 2);
    expect(tester.takeException(), isNull);
  });
}
