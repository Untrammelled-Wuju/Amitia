import 'package:amitia_app/features/conversation/rendering/amrp.dart';
import 'package:amitia_app/features/conversation/rendering/markdown/markdown_parser.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('旧消息可以归一化为 AMRP Tool Block', () {
    final message = AmrpMessage.fromChatMessage(
      ChatMessage(
        id: 'm1',
        role: MessageRole.assistant,
        type: MessageType.toolCall,
        content: '',
        time: DateTime(2026, 9, 20),
        toolName: 'read_file',
        toolResult: '{"ok":true}',
      ),
    );

    expect(message.blocks, hasLength(1));
    final block = message.blocks.single as AmrpToolBlock;
    expect(block.name, 'read_file');
    expect(block.status, AmrpToolStatus.success);
    expect(block.result, '{"ok":true}');
    expect(message.plainText, contains('read_file'));
  });

  test('Markdown Fence 按 Renderer 类型拆分', () {
    final segments = splitAmitiaMarkdown([
      '正文',
      '```dart',
      'void main() {}',
      '```',
      '```diff',
      '- old',
      '+ new',
      '```',
      '```terminal',
      r'$ flutter test',
      '```',
      '```mermaid',
      'graph TD',
      'A --> B',
      '```',
      '```html-preview',
      '<h1>Hello</h1>',
      '```',
      r'$$',
      'E=mc^2',
      r'$$',
    ].join('\n'));

    expect(
      segments.map((segment) => segment.type).toList(),
      [
        AmitiaMarkdownSegmentType.markdown,
        AmitiaMarkdownSegmentType.code,
        AmitiaMarkdownSegmentType.diff,
        AmitiaMarkdownSegmentType.terminal,
        AmitiaMarkdownSegmentType.mermaid,
        AmitiaMarkdownSegmentType.htmlPreview,
        AmitiaMarkdownSegmentType.latex,
      ],
    );
    expect(segments[1].language, 'dart');
  });

  test('未闭合 Fence 保持流式状态', () {
    final segments = splitAmitiaMarkdown('```dart\nclass Test {');
    expect(segments.single.type, AmitiaMarkdownSegmentType.code);
    expect(segments.single.streaming, isTrue);
  });
}
