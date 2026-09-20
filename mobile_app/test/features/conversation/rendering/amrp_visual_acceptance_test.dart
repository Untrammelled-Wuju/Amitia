import 'dart:io';
import 'dart:ui' as ui;

import 'package:amitia_app/features/conversation/rendering/amitia_message_theme.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_view.dart';
import 'package:amitia_app/features/conversation/rendering/amrp.dart';
import 'package:amitia_app/features/conversation/rendering/rich_blocks/amitia_rich_blocks.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  setUpAll(() async {
    final fontData = File('C:/Windows/Fonts/simhei.ttf').readAsBytesSync();
    final loader = FontLoader('AmitiaTest')
      ..addFont(Future.value(ByteData.sublistView(fontData)));
    await loader.load();
    final monoData = File('C:/Windows/Fonts/consola.ttf').readAsBytesSync();
    final monoLoader = FontLoader('monospace')
      ..addFont(Future.value(ByteData.sublistView(monoData)));
    await monoLoader.load();
    final iconData = File(
      'C:/Code/flutter/SDK/flutter_windows_3.38.7-stable/flutter/bin/cache/'
      'artifacts/material_fonts/materialicons-regular.otf',
    ).readAsBytesSync();
    final iconLoader = FontLoader('MaterialIcons')
      ..addFont(Future.value(ByteData.sublistView(iconData)));
    await iconLoader.load();
  });

  final viewports = <(int, int)>[
    (1440, 900),
    (1280, 800),
    (1024, 768),
    (430, 932),
    (390, 844),
    (375, 812),
    (360, 800),
  ];

  for (final dark in [false, true]) {
    for (final viewport in viewports) {
      testWidgets(
        'Flutter AMRP ${dark ? 'dark' : 'light'} ${viewport.$1}x${viewport.$2}',
        (tester) async {
          await tester.binding.setSurfaceSize(
            Size(viewport.$1.toDouble(), viewport.$2.toDouble()),
          );
          tester.view.devicePixelRatio = 1;
          final boundaryKey = GlobalKey();
          await tester.pumpWidget(
            MaterialApp(
              debugShowCheckedModeBanner: false,
              theme: ThemeData(
                brightness: dark ? Brightness.dark : Brightness.light,
                fontFamily: 'AmitiaTest',
                extensions: [
                  dark ? AmitiaMessageTheme.dark : AmitiaMessageTheme.light,
                ],
              ),
              home: Scaffold(
                backgroundColor: dark
                    ? AmitiaMessageTheme.dark.background
                    : AmitiaMessageTheme.light.background,
                body: RepaintBoundary(
                  key: boundaryKey,
                  child: ColoredBox(
                    color: dark
                        ? AmitiaMessageTheme.dark.background
                        : AmitiaMessageTheme.light.background,
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.fromLTRB(12, 22, 12, 80),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          AmitiaMessageView(
                            message: _message(),
                            characterName: '林澈',
                            avatarInitial: '澈',
                            avatarColor: '#7060E8',
                            toolBlocks: const [
                              AmrpToolBlock(
                                id: 'tool-running',
                                name: '执行命令',
                                arguments: {
                                  'cwd': '/workspace/app',
                                  'timeout': 120,
                                },
                                status: AmrpToolStatus.running,
                              ),
                            ],
                          ),
                          Padding(
                            padding: const EdgeInsets.only(left: 42),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.stretch,
                              children: const [
                                AmitiaAgentTaskBlock(
                                  block: AmrpAgentTaskBlock(
                                    id: 'agent',
                                    title: '修复消息渲染链路',
                                    status: AmrpAgentStepStatus.running,
                                    progress: 68,
                                    elapsed: '3.2s',
                                    steps: [
                                      AmrpAgentStep(
                                        id: '1',
                                        title: '读取聊天相关源码',
                                        status: AmrpAgentStepStatus.done,
                                        meta: '完成',
                                      ),
                                      AmrpAgentStep(
                                        id: '2',
                                        title: '定位重复拼接逻辑',
                                        status: AmrpAgentStepStatus.done,
                                        meta: '完成',
                                      ),
                                      AmrpAgentStep(
                                        id: '3',
                                        title: '修改双端 Renderer',
                                        status: AmrpAgentStepStatus.running,
                                        meta: '进行中',
                                      ),
                                    ],
                                  ),
                                ),
                                AmitiaArtifactBlock(
                                  block: AmrpArtifactBlock(
                                    id: 'artifact',
                                    title: '消息渲染接缝报告',
                                    artifactKind: 'Markdown',
                                    mimeType: 'text/markdown',
                                    content:
                                        '# Artifact\n\nRenderer registry ready.',
                                    size: 28600,
                                  ),
                                ),
                              ],
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
          await tester.pump(const Duration(seconds: 2));
          await tester.runAsync(() async {
            final boundary =
                boundaryKey.currentContext!.findRenderObject()
                    as RenderRepaintBoundary;
            final image = await boundary.toImage(pixelRatio: 1);
            final data = await image.toByteData(format: ui.ImageByteFormat.png);
            final outDir = Directory('../artifacts/visual-validation')
              ..createSync(recursive: true);
            final file = File(
              '${outDir.path}/flutter-${dark ? 'dark' : 'light'}-'
              '${viewport.$1}x${viewport.$2}.png',
            );
            file.writeAsBytesSync(data!.buffer.asUint8List());
          });
          await tester.binding.setSurfaceSize(null);
        },
      );
    }
  }
}

ChatMessage _message() {
  return ChatMessage(
    id: 'acceptance-message',
    role: MessageRole.assistant,
    type: MessageType.text,
    reasoningContent: '先确认消息聚合入口，再重构双端 Renderer。',
    content: [
      '# Amitia Message Rendering Engine',
      '',
      '同一条 AI 回复里包含 **正文**、*斜体*、~~删除线~~、[安全链接](https://example.com)、行内代码 `const ok = true` 和引用 [1]。',
      '',
      '## Code',
      '',
      '```ts filename=stream_message_store.ts',
      'export function applyChunk(messageId: string, delta: string) {',
      '  const current = streamBuffers.get(messageId) ?? "";',
      '  const content = current + delta;',
      '  streamBuffers.set(messageId, content);',
      '  patchMessage(messageId, { content });',
      '}',
      '```',
      '',
      '```diff filename=ChatBubble.vue',
      '- localText.value += event.delta',
      '- updateMessage(event.id, localText.value)',
      '+ streamStore.applyChunk(event.id, event.delta)',
      '  scrollToBottom()',
      '```',
      '',
      '```terminal filename=Terminal',
      r'$ pnpm run build',
      'vite v7.1.2 building for production...',
      '✓ 184 modules transformed.',
      "error TS2322: Type 'null' is not assignable to type 'string'.",
      '```',
      '',
      '## 表格与任务',
      '',
      '| 内容 | 表现 | 是否单独成卡 |',
      '| --- | --- | --- |',
      '| 正文 / Markdown | 直接排版 | 否 |',
      '| 代码 / Diff | 深色嵌入块 | 否 |',
      '',
      '- [x] 已完成任务',
      '- [ ] 未完成任务',
      '',
      '> 逻辑上是一整条回复，视觉上没有 AI 外层大卡片。',
      '',
      '行内公式 \$E=mc^2\$，块级公式：',
      '',
      r'$$',
      r'\int_0^\infty e^{-x^2}\,dx=\frac{\sqrt{\pi}}{2}',
      r'$$',
      '',
      '```mermaid',
      'graph LR',
      'A[LLM Stream] --> B[Renderer]',
      'B --> C[Unified Message UI]',
    ].join('\n'),
    time: DateTime(2026, 9, 20, 20, 41),
  );
}
