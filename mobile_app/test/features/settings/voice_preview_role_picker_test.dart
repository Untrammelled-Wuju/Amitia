import 'package:amitia_app/features/settings/presentation/widgets/voice_preview_role_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('单角色直接返回，不弹出选择', (tester) async {
    String? selected;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () async {
              selected = await pickVoicePreviewRole(context, [
                {'id': 'one', 'name': '角色一'},
              ]);
            },
            child: const Text('选择'),
          ),
        ),
      ),
    );

    await tester.tap(find.text('选择'));
    await tester.pumpAndSettle();

    expect(selected, 'one');
    expect(find.text('选择试听角色'), findsNothing);
  });

  testWidgets('多角色必须明确确认后才返回', (tester) async {
    String? selected;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () async {
              selected = await pickVoicePreviewRole(context, [
                {'id': 'one', 'name': '角色一'},
                {'id': 'two', 'name': '角色二'},
              ], preferredRoleId: 'two');
            },
            child: const Text('选择'),
          ),
        ),
      ),
    );

    await tester.tap(find.text('选择'));
    await tester.pumpAndSettle();

    expect(find.text('选择试听角色'), findsOneWidget);
    expect(find.text('角色二'), findsOneWidget);
    expect(selected, isNull);

    await tester.tap(find.byKey(const Key('voice-preview-role-confirm')));
    await tester.pumpAndSettle();

    expect(selected, 'two');
  });

  testWidgets('多角色无当前角色时必须选择', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => pickVoicePreviewRole(context, [
              {'id': 'one', 'name': '角色一'},
              {'id': 'two', 'name': '角色二'},
            ]),
            child: const Text('选择'),
          ),
        ),
      ),
    );

    await tester.tap(find.text('选择'));
    await tester.pumpAndSettle();

    final confirm = tester.widget<FilledButton>(
      find.byKey(const Key('voice-preview-role-confirm')),
    );
    expect(confirm.onPressed, isNull);
  });
}
