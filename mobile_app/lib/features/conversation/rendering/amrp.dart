import '../../../shared/models/models.dart';

enum AmrpMessageRole { user, assistant, system }

enum AmrpMessageState {
  queued,
  streaming,
  completed,
  interrupted,
  failed,
  cancelled,
}

enum AmrpToolStatus { queued, running, success, failed, cancelled }

enum AmrpAssetStatus { loading, ready, failed }

enum AmrpAgentStepStatus { pending, running, done, failed }

class AmrpCharacter {
  final String id;
  final String name;
  final String avatar;

  const AmrpCharacter({
    this.id = '',
    this.name = 'Amitia',
    this.avatar = '',
  });
}

class AmrpThinkingBlock {
  final String content;
  final AmrpMessageState state;
  final Duration? duration;

  const AmrpThinkingBlock({
    required this.content,
    this.state = AmrpMessageState.completed,
    this.duration,
  });
}

class AmrpCitationSource {
  final String id;
  final String title;
  final String url;
  final String fileId;
  final String snippet;

  const AmrpCitationSource({
    required this.id,
    required this.title,
    this.url = '',
    this.fileId = '',
    this.snippet = '',
  });
}

sealed class AmrpRichBlock {
  final String id;

  const AmrpRichBlock({required this.id});
}

class AmrpToolBlock extends AmrpRichBlock {
  final String name;
  final Object? arguments;
  final Object? result;
  final Duration? duration;
  final String error;
  final AmrpToolStatus status;

  const AmrpToolBlock({
    required super.id,
    required this.name,
    this.arguments,
    this.result,
    this.duration,
    this.error = '',
    this.status = AmrpToolStatus.success,
  });
}

class AmrpFileBlock extends AmrpRichBlock {
  final String name;
  final String mimeType;
  final int size;
  final String url;
  final AmrpAssetStatus status;
  final String error;

  const AmrpFileBlock({
    required super.id,
    required this.name,
    this.mimeType = '',
    this.size = 0,
    this.url = '',
    this.status = AmrpAssetStatus.ready,
    this.error = '',
  });
}

class AmrpImageBlock extends AmrpRichBlock {
  final String url;
  final String alt;
  final String mimeType;
  final bool animated;
  final double width;
  final double height;
  final AmrpAssetStatus status;

  const AmrpImageBlock({
    required super.id,
    required this.url,
    this.alt = '',
    this.mimeType = '',
    this.animated = false,
    this.width = 0,
    this.height = 0,
    this.status = AmrpAssetStatus.loading,
  });
}

class AmrpAudioBlock extends AmrpRichBlock {
  final String url;
  final String title;
  final Duration? duration;
  final AmrpAssetStatus status;

  const AmrpAudioBlock({
    required super.id,
    required this.url,
    this.title = '语音',
    this.duration,
    this.status = AmrpAssetStatus.loading,
  });
}

class AmrpVideoBlock extends AmrpRichBlock {
  final String url;
  final String title;
  final String poster;
  final Duration? duration;
  final AmrpAssetStatus status;

  const AmrpVideoBlock({
    required super.id,
    required this.url,
    this.title = '视频',
    this.poster = '',
    this.duration,
    this.status = AmrpAssetStatus.loading,
  });
}

class AmrpArtifactBlock extends AmrpRichBlock {
  final String artifactKind;
  final String title;
  final String mimeType;
  final String content;
  final String fileId;
  final String url;
  final int size;

  const AmrpArtifactBlock({
    required super.id,
    required this.title,
    this.artifactKind = 'document',
    this.mimeType = '',
    this.content = '',
    this.fileId = '',
    this.url = '',
    this.size = 0,
  });
}

class AmrpAgentStep {
  final String id;
  final String title;
  final AmrpAgentStepStatus status;
  final String meta;

  const AmrpAgentStep({
    required this.id,
    required this.title,
    this.status = AmrpAgentStepStatus.pending,
    this.meta = '',
  });
}

class AmrpAgentTaskBlock extends AmrpRichBlock {
  final String title;
  final AmrpAgentStepStatus status;
  final List<AmrpAgentStep> steps;
  final int progress;
  final String elapsed;
  final String error;

  const AmrpAgentTaskBlock({
    required super.id,
    required this.title,
    this.status = AmrpAgentStepStatus.pending,
    this.steps = const <AmrpAgentStep>[],
    this.progress = 0,
    this.elapsed = '',
    this.error = '',
  });
}

class AmrpExtensionBlock extends AmrpRichBlock {
  final String rendererId;
  final int version;
  final Object? payload;

  const AmrpExtensionBlock({
    required super.id,
    required this.rendererId,
    this.version = 1,
    this.payload,
  });
}

class AmrpUnknownBlock extends AmrpRichBlock {
  final String type;
  final Object? payload;

  const AmrpUnknownBlock({
    required super.id,
    required this.type,
    this.payload,
  });
}

class AmrpMessage {
  final String id;
  final String conversationId;
  final AmrpMessageRole role;
  final AmrpCharacter character;
  final String markdown;
  final AmrpThinkingBlock? thinking;
  final List<AmrpRichBlock> blocks;
  final List<AmrpCitationSource> sources;
  final AmrpMessageState state;
  final DateTime createdAt;
  final ChatMessage raw;

  const AmrpMessage({
    required this.id,
    required this.conversationId,
    required this.role,
    required this.character,
    required this.markdown,
    required this.blocks,
    required this.sources,
    required this.state,
    required this.createdAt,
    required this.raw,
    this.thinking,
  });

  static AmrpMessage fromChatMessage(
    ChatMessage message, {
    AmrpCharacter character = const AmrpCharacter(),
  }) {
    return AmrpMessage(
      id: message.id,
      conversationId: '',
      role: switch (message.role) {
        MessageRole.user => AmrpMessageRole.user,
        MessageRole.system => AmrpMessageRole.system,
        MessageRole.assistant => AmrpMessageRole.assistant,
      },
      character: character,
      markdown: _markdownFor(message),
      thinking: message.reasoningContent.trim().isEmpty
          ? null
          : AmrpThinkingBlock(
              content: message.reasoningContent,
              state: _stateFor(message),
            ),
      blocks: _blocksFor(message),
      sources: const <AmrpCitationSource>[],
      state: _stateFor(message),
      createdAt: message.time,
      raw: message,
    );
  }

  static AmrpMessageState _stateFor(ChatMessage message) {
    return switch (message.status) {
      MessageStatus.sending => AmrpMessageState.streaming,
      MessageStatus.error => AmrpMessageState.failed,
      MessageStatus.sent || MessageStatus.delivered => AmrpMessageState.completed,
    };
  }

  static String _markdownFor(ChatMessage message) {
    if (message.type == MessageType.toolCall ||
        message.type == MessageType.agentTask ||
        message.type == MessageType.systemNotice) {
      return '';
    }
    if (message.type == MessageType.image && message.content == '[图片]') {
      return '';
    }
    if (message.type == MessageType.video && message.content == '[视频]') {
      return '';
    }
    if (message.type == MessageType.audio && message.content == '[语音]') {
      return '';
    }
    return message.content;
  }

  static List<AmrpRichBlock> _blocksFor(ChatMessage message) {
    final blocks = <AmrpRichBlock>[];
    switch (message.type) {
      case MessageType.toolCall:
        blocks.add(
          AmrpToolBlock(
            id: '${message.id}:tool',
            name: message.toolName?.trim().isNotEmpty == true
                ? message.toolName!.trim()
                : '工具调用',
            result: message.toolResult,
            error: message.status == MessageStatus.error
                ? (message.toolResult ?? '')
                : '',
            status: message.status == MessageStatus.error
                ? AmrpToolStatus.failed
                : message.status == MessageStatus.sending
                ? AmrpToolStatus.running
                : AmrpToolStatus.success,
          ),
        );
        break;
      case MessageType.agentTask:
        final steps = (message.agentTaskSteps ?? const <String>[])
            .asMap()
            .entries
            .map(
              (entry) => AmrpAgentStep(
                id: '${message.id}:step:${entry.key}',
                title: entry.value,
                status: _agentStatusFromProgress(
                  entry.key,
                  message.agentTaskProgress ?? 0,
                ),
              ),
            )
            .toList(growable: false);
        blocks.add(
          AmrpAgentTaskBlock(
            id: '${message.id}:agent',
            title: message.agentTaskTitle?.trim().isNotEmpty == true
                ? message.agentTaskTitle!.trim()
                : 'Agent Task',
            status: message.status == MessageStatus.error
                ? AmrpAgentStepStatus.failed
                : message.status == MessageStatus.sending
                ? AmrpAgentStepStatus.running
                : AmrpAgentStepStatus.done,
            steps: steps,
            progress: message.agentTaskProgress ?? 0,
            elapsed: message.agentTaskElapsed ?? '',
          ),
        );
        break;
      case MessageType.file:
        blocks.add(
          AmrpFileBlock(
            id: '${message.id}:file',
            name: message.fileName?.trim().isNotEmpty == true
                ? message.fileName!.trim()
                : '未命名文件',
            mimeType: message.mimeType ?? '',
            size: (message.fileSizeKB ?? 0) * 1024,
            url: message.resourceUri ?? '',
            status: message.status == MessageStatus.error
                ? AmrpAssetStatus.failed
                : AmrpAssetStatus.ready,
            error: message.status == MessageStatus.error ? message.content : '',
          ),
        );
        break;
      case MessageType.image:
        final url = (message.mediaUrl ?? message.resourceUri ?? '').trim();
        if (url.isNotEmpty) {
          blocks.add(
            AmrpImageBlock(
              id: '${message.id}:image',
              url: url,
              alt: message.fileName ?? '',
              mimeType: message.mimeType ?? '',
            ),
          );
        }
        break;
      case MessageType.audio:
        final url = (message.mediaUrl ?? message.resourceUri ?? '').trim();
        if (url.isNotEmpty) {
          blocks.add(
            AmrpAudioBlock(
              id: '${message.id}:audio',
              url: url,
              title: message.fileName ?? '语音',
              duration: message.durationMs == null
                  ? null
                  : Duration(milliseconds: message.durationMs!),
            ),
          );
        }
        break;
      case MessageType.video:
        final url = (message.mediaUrl ?? message.resourceUri ?? '').trim();
        if (url.isNotEmpty) {
          blocks.add(
            AmrpVideoBlock(
              id: '${message.id}:video',
              url: url,
              title: message.fileName ?? '视频',
              duration: message.durationMs == null
                  ? null
                  : Duration(milliseconds: message.durationMs!),
            ),
          );
        }
        break;
      case MessageType.text:
      case MessageType.code:
      case MessageType.emote:
      case MessageType.systemNotice:
        break;
    }
    return blocks;
  }

  static AmrpAgentStepStatus _agentStatusFromProgress(int index, int progress) {
    if (progress <= 0) return AmrpAgentStepStatus.pending;
    final completedSteps = (progress / 25).floor();
    if (index < completedSteps) return AmrpAgentStepStatus.done;
    if (index == completedSteps) return AmrpAgentStepStatus.running;
    return AmrpAgentStepStatus.pending;
  }

  String get plainText {
    final values = <String>[
      markdown,
      if (thinking != null) thinking!.content,
      ...blocks.map(_blockText),
    ];
    return values.where((value) => value.trim().isNotEmpty).join('\n\n');
  }

  static String _blockText(AmrpRichBlock block) {
    return switch (block) {
      AmrpToolBlock() =>
        '${block.name}\n${_stringify(block.arguments)}\n${_stringify(block.result)}',
      AmrpFileBlock() => '${block.name} ${block.mimeType} ${block.url}',
      AmrpImageBlock() => block.alt.isNotEmpty ? block.alt : block.url,
      AmrpAudioBlock() => '${block.title} ${block.url}',
      AmrpVideoBlock() => '${block.title} ${block.url}',
      AmrpArtifactBlock() => '${block.title}\n${block.content}',
      AmrpAgentTaskBlock() =>
        '${block.title}\n${block.steps.map((step) => step.title).join('\n')}',
      AmrpExtensionBlock() => '${block.rendererId}\n${_stringify(block.payload)}',
      AmrpUnknownBlock() => _stringify(block.payload),
    };
  }

  static String _stringify(Object? value) {
    if (value == null) return '';
    if (value is String) return value;
    return value.toString();
  }
}

