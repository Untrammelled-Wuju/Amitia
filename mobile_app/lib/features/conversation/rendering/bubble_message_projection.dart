import '../../../core/models/conversation.dart';
import 'amrp.dart';

const bubbleSeparator = '[AMITIA_BR]';

bool isBubbleContentComplete(String status) => const {
  'completed',
  'complete',
  'sent',
  'delivered',
  'success',
  'succeeded',
  'failed',
  'error',
  'interrupted',
  'cancelled',
  'canceled',
}.contains(status.toLowerCase());

List<String> splitBubbleText(String content, {required bool complete}) {
  final result = <String>[];
  var start = 0;
  var code = '';
  for (var i = 0; i < content.length;) {
    final char = content[i];
    if (char == '`' || char == '~') {
      var end = i + 1;
      while (end < content.length && content[end] == char) {
        end++;
      }
      final delimiter = content.substring(i, end);
      if (char == '`' || delimiter.length >= 3) {
        if (code.isEmpty) {
          code = delimiter;
        } else if (code[0] == char && delimiter.length >= code.length) {
          code = '';
        }
      }
      i = end;
      continue;
    }
    if (code.isEmpty && content.startsWith(bubbleSeparator, i)) {
      final text = content.substring(start, i).trim();
      if (text.isNotEmpty) result.add(text);
      i += bubbleSeparator.length;
      start = i;
      continue;
    }
    i++;
  }
  if (complete) {
    var tail = content.substring(start).trim();
    if (code.isEmpty) {
      for (var size = bubbleSeparator.length - 1; size >= 4; size--) {
        if (tail.endsWith(bubbleSeparator.substring(0, size))) {
          tail = tail.substring(0, tail.length - size).trim();
          break;
        }
      }
    }
    if (tail.isNotEmpty) result.add(tail);
  }
  return result;
}

sealed class BubbleContentItem {
  final String key;
  const BubbleContentItem(this.key);
}

String flowBubbleText(String content) => content.contains(bubbleSeparator)
    ? splitBubbleText(content, complete: true).join('\n\n')
    : content;

class BubbleText extends BubbleContentItem {
  final String content;
  const BubbleText(super.key, this.content);
}

class BubbleThinking extends BubbleContentItem {
  final AmrpThinkingBlock block;
  const BubbleThinking(super.key, this.block);
}

class BubbleTools extends BubbleContentItem {
  final List<AssistantTurnItemDto> items;
  const BubbleTools(super.key, this.items);
}

class BubbleRichContent extends BubbleContentItem {
  final AmrpRichBlock block;
  const BubbleRichContent(super.key, this.block);
}

List<BubbleContentItem> projectBubbleContent({
  required AmrpMessage message,
  AssistantTurnDto? turn,
  List<AmrpToolBlock> toolBlocks = const [],
}) {
  final output = <BubbleContentItem>[];
  final turnComplete = turn != null && isBubbleContentComplete(turn.status);
  if (turn != null && turn.items.isNotEmpty) {
    final ordered = [...turn.items]
      ..sort((a, b) => a.sequence.compareTo(b.sequence));
    final tools = <String, List<AssistantTurnItemDto>>{};
    for (final item in ordered) {
      if (item.type == 'tool_call' || item.type == 'tool_result') {
        (tools[item.callId.isEmpty ? item.id : item.callId] ??= []).add(item);
      }
    }
    final emittedTools = <String>{};
    for (final item in ordered) {
      final complete = turnComplete || isBubbleContentComplete(item.status);
      if (item.type == 'text') {
        final texts = splitBubbleText(item.content, complete: complete);
        for (var i = 0; i < texts.length; i++) {
          output.add(BubbleText('${item.id}:text:$i', texts[i]));
        }
      } else if (item.type == 'reasoning') {
        output.add(
          BubbleThinking(
            item.id,
            AmrpThinkingBlock(
              content: complete ? item.content : '',
              state: complete
                  ? AmrpMessageState.completed
                  : AmrpMessageState.streaming,
              duration: item.durationMs > 0
                  ? Duration(milliseconds: item.durationMs)
                  : null,
            ),
          ),
        );
      } else if (item.type == 'tool_call' || item.type == 'tool_result') {
        final key = item.callId.isEmpty ? item.id : item.callId;
        if (emittedTools.add(key)) {
          output.add(BubbleTools('tool:$key', tools[key]!));
        }
      }
    }
  } else {
    final complete =
        message.role != AmrpMessageRole.assistant ||
        isBubbleContentComplete(message.state.name) ||
        turnComplete;
    if (message.thinking != null) {
      output.add(
        BubbleThinking(
          '${message.id}:thinking',
          AmrpThinkingBlock(
            content: complete ? message.thinking!.content : '',
            state: complete
                ? AmrpMessageState.completed
                : AmrpMessageState.streaming,
            duration: message.thinking!.duration,
          ),
        ),
      );
    }
    final texts = message.role == AmrpMessageRole.user
        ? (message.markdown.trim().isEmpty ? <String>[] : [message.markdown])
        : splitBubbleText(message.markdown, complete: complete);
    for (var i = 0; i < texts.length; i++) {
      output.add(BubbleText('${message.id}:text:$i', texts[i]));
    }
  }
  for (final block in [...message.blocks, ...toolBlocks]) {
    if (turn?.items.isNotEmpty == true && block is AmrpToolBlock) continue;
    output.add(BubbleRichContent('${message.id}:block:${block.id}', block));
  }
  return output;
}
