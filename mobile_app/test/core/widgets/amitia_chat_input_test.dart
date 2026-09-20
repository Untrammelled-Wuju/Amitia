import 'dart:io';

import 'package:amitia_app/core/widgets/amitia_message.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('voice composer keeps text vertically aligned with input text', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: AmitiaChatInput(onSend: (_) {})),
      ),
    );

    final textFieldCenter = tester.getCenter(find.text('发消息…')).dy;

    await tester.tap(find.byTooltip('按住说话'));
    await tester.pumpAndSettle();

    final voiceTextCenter = tester.getCenter(find.text('按住说话')).dy;
    final composerTop = tester
        .getTopLeft(find.byKey(const ValueKey('chat-composer-surface')))
        .dy;
    final voiceButtonTop = tester
        .getTopLeft(find.byKey(const ValueKey('hold-to-talk-button')))
        .dy;
    final voiceButtonHeight = tester
        .getSize(find.byKey(const ValueKey('hold-to-talk-button')))
        .height;

    expect(voiceTextCenter, closeTo(textFieldCenter, 0.5));
    expect(voiceButtonTop - composerTop, greaterThanOrEqualTo(6));
    expect(voiceButtonHeight, closeTo(44, 0.1));
  });

  testWidgets('composer border does not change when input is focused', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: AmitiaChatInput(onSend: (_) {})),
      ),
    );

    BoxDecoration decoration() =>
        tester
                .widget<Container>(
                  find.byKey(const ValueKey('chat-composer-surface')),
                )
                .decoration!
            as BoxDecoration;

    final before = decoration().border;
    await tester.tap(find.byType(TextField));
    await tester.pump();
    final after = decoration().border;

    expect(after, before);
  });

  testWidgets('composer trailing actions keep a small right inset', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(375, 800);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: AmitiaChatInput(
            onSend: (_) {},
            workspaceSelector: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 90),
              child: Container(
                height: 31,
                padding: const EdgeInsets.symmetric(horizontal: 7),
                child: const Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.folder_outlined, size: 16),
                    SizedBox(width: 5),
                    Flexible(
                      child: Text(
                        '选择项目',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(fontSize: 12),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );

    final surface = tester.getRect(
      find.byKey(const ValueKey('chat-composer-surface')),
    );
    final add = tester.getRect(
      find.byKey(const ValueKey('composer-add-button')),
    );
    final workspace = tester.getRect(
      find.byKey(const ValueKey('composer-workspace-selector')),
    );
    final permission = tester.getRect(
      find.byKey(const ValueKey('composer-permission-button')),
    );
    final voice = tester.getRect(
      find.byKey(const ValueKey('composer-trailing-button')),
    );
    final trigger = tester.getRect(
      find.byKey(const ValueKey('composer-model-trigger')),
    );

    final addInset = add.left - surface.left;
    final voiceInset = surface.right - voice.right;

    expect(addInset, closeTo(10.8, 0.5));
    expect(voiceInset, closeTo(10.8, 0.5));
    expect(permission.left - add.right, closeTo(4, 0.5));
    expect(workspace.left - permission.right, closeTo(4, 0.5));
    expect(trigger.left, greaterThanOrEqualTo(workspace.right));
    expect(voice.left - trigger.right, closeTo(4, 0.5));
    expect(
      tester.renderObject<RenderParagraph>(find.text('选择项目')).didExceedMaxLines,
      isFalse,
    );

    await tester.enterText(find.byType(TextField), '测试');
    await tester.pump();

    final send = tester.getRect(
      find.byKey(const ValueKey('composer-send-button')),
    );
    final sendInset = surface.right - send.right;

    expect(sendInset, closeTo(10.8, 0.5));
    expect(send.left - trigger.right, closeTo(4, 0.5));
  });

  test('composer keeps a lower bottom inset when the keyboard is closed', () {
    final source = File(
      'lib/core/widgets/amitia_message.dart',
    ).readAsStringSync();

    expect(
      source,
      contains('padding: const EdgeInsets.fromLTRB(10, 8, 10, 4)'),
    );
  });

  testWidgets('composer text and hold-to-talk use the compact size', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: AmitiaChatInput(onSend: (_) {})),
      ),
    );

    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.style?.fontSize, 15);
    expect(field.decoration?.hintStyle?.fontSize, 15);

    await tester.tap(find.byTooltip('按住说话'));
    await tester.pumpAndSettle();

    final holdToTalk = tester.widget<Text>(find.text('按住说话'));
    expect(holdToTalk.style?.fontSize, 15);
  });

  testWidgets(
    'reasoning slider previews while dragging and commits on release',
    (tester) async {
      final previews = <String>[];
      final commits = <String>[];
      final modeCommits = <bool>[];
      var effort = 'medium';
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Align(
              alignment: Alignment.bottomCenter,
              child: StatefulBuilder(
                builder: (context, setState) {
                  return AmitiaChatInput(
                    onSend: (_) {},
                    models: const <Map<String, dynamic>>[
                      <String, dynamic>{
                        'id': 7,
                        'name': 'gpt-5',
                        'modelName': 'gpt-5',
                        'apiType': 'openai',
                        'supportsReasoning': true,
                        'defaultReasoningEffort': 'medium',
                      },
                    ],
                    selectedModelId: 7,
                    reasoningEffort: effort,
                    reasoningEnabled: true,
                    onModelPreviewChanged: (_, nextEffort, _) {
                      previews.add(nextEffort);
                      setState(() => effort = nextEffort);
                    },
                    onModelChanged: (_, nextEffort, enabled) {
                      commits.add(nextEffort);
                      modeCommits.add(enabled);
                      setState(() => effort = nextEffort);
                    },
                  );
                },
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.byKey(const ValueKey('composer-reasoning-label')));
      await tester.pumpAndSettle();
      final slider = find.byType(Slider);
      expect(slider, findsOneWidget);
      expect(find.byType(Switch), findsNothing);

      final sliderRect = tester.getRect(slider);
      final gesture = await tester.startGesture(
        Offset(sliderRect.left + sliderRect.width * 0.37, sliderRect.center.dy),
      );
      await gesture.moveBy(Offset(sliderRect.width * 0.28, 0));
      await tester.pump();

      expect(previews, isNotEmpty);
      expect(commits, isEmpty);
      expect(
        tester
            .widget<Text>(
              find.byKey(const ValueKey('composer-reasoning-label')),
            )
            .data,
        '高',
      );

      await gesture.up();
      await tester.pumpAndSettle();

      expect(commits, isNotEmpty);

      await tester.tap(find.text('思考'));
      await tester.pumpAndSettle();

      expect(find.text('选择思考模式'), findsOneWidget);
      expect(find.text('不支持'), findsOneWidget);

      await tester.tap(find.text('不支持'));
      await tester.pumpAndSettle();

      expect(modeCommits.last, isFalse);
    },
  );

  testWidgets('model selection page keeps model entries visible', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Align(
            alignment: Alignment.bottomCenter,
            child: AmitiaChatInput(
              onSend: (_) {},
              models: const <Map<String, dynamic>>[
                <String, dynamic>{
                  'id': 7,
                  'name': 'GPT-5',
                  'modelName': 'gpt-5',
                  'apiType': 'openai',
                  'supportsReasoning': true,
                  'defaultReasoningEffort': 'high',
                },
              ],
              selectedModelId: 7,
              reasoningEffort: 'high',
              reasoningEnabled: true,
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.byKey(const ValueKey('composer-reasoning-label')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('模型'));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('选择模型'), findsOneWidget);
    expect(find.text('gpt-5'), findsOneWidget);
    expect(find.textContaining('GPT-5 · openai'), findsOneWidget);
  });
}
