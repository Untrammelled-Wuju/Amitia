import 'dart:async';

import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_motion.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../shared/models/models.dart';
import 'amitia_button.dart';
import 'amitia_misc.dart';

const _mockPrefix = '__mock:';

enum _MockType { image, video, audio, emote, code }

class _MockKind {
  final _MockType type;
  final List<String> args;
  const _MockKind(this.type, this.args);
}

_MockKind? _parseMock(String content) {
  if (!content.startsWith(_mockPrefix)) return null;
  final body = content.substring(_mockPrefix.length);
  final firstPipe = body.indexOf('|');
  if (firstPipe == -1) return null;
  final type = body.substring(0, firstPipe);
  final rest = body.substring(firstPipe + 1);
  switch (type) {
    case 'image':
      return _MockKind(_MockType.image, [rest]);
    case 'video':
      return _MockKind(_MockType.video, [rest]);
    case 'audio':
      return _MockKind(_MockType.audio, [rest]);
    case 'emote':
      final p = rest.indexOf('|');
      if (p == -1) return _MockKind(_MockType.emote, [rest, '']);
      return _MockKind(_MockType.emote, [
        rest.substring(0, p),
        rest.substring(p + 1),
      ]);
    case 'code':
      final p = rest.indexOf('|');
      if (p == -1) return _MockKind(_MockType.code, [rest, '']);
      return _MockKind(_MockType.code, [
        rest.substring(0, p),
        rest.substring(p + 1),
      ]);
    default:
      return null;
  }
}

String mockImagePayload(String name) => '$_mockPrefix$_image|$name';
String mockVideoPayload(String title) => '$_mockPrefix$_video|$title';
String mockAudioPayload(String duration) => '$_mockPrefix$_audio|$duration';
String mockEmotePayload(String emoji, String name) =>
    '$_mockPrefix$_emote|$emoji|$name';
String mockCodePayload(String lang, String body) =>
    '$_mockPrefix$_code|$lang|$body';

const _image = 'image';
const _video = 'video';
const _audio = 'audio';
const _emote = 'emote';
const _code = 'code';

class AmitiaMessageBubble extends StatelessWidget {
  final ChatMessage message;
  final bool showAvatar;
  final String? avatarInitial;
  final String? avatarColor;
  final String? characterName;
  final VoidCallback? onRetry;
  final VoidCallback? onAgentTaskTap;
  final VoidCallback? onPauseAgentTask;
  final VoidCallback? onResumeAgentTask;
  final String? agentTaskStatusLabel;

  const AmitiaMessageBubble({
    super.key,
    required this.message,
    this.showAvatar = false,
    this.avatarInitial,
    this.avatarColor,
    this.characterName,
    this.onRetry,
    this.onAgentTaskTap,
    this.onPauseAgentTask,
    this.onResumeAgentTask,
    this.agentTaskStatusLabel,
  });

  @override
  Widget build(BuildContext context) {
    if (message.type == MessageType.systemNotice) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.sm),
        child: Center(
          child: Text(
            message.content,
            style: TextStyle(fontSize: 11, color: context.textTertiary),
          ),
        ),
      );
    }

    if (message.type == MessageType.agentTask) {
      return _AgentTaskMessage(
        message: message,
        onAgentTaskTap: onAgentTaskTap,
        onPauseAgentTask: onPauseAgentTask,
        onResumeAgentTask: onResumeAgentTask,
        statusLabel: agentTaskStatusLabel ?? '运行中',
      );
    }

    if (message.type == MessageType.toolCall) {
      return _ToolCallMessage(message: message);
    }

    final isUser = message.role == MessageRole.user;
    final mock = _parseMock(message.content);

    return Padding(
      padding: EdgeInsets.only(
        left: isUser ? 60 : AppSpacing.lg,
        right: isUser ? AppSpacing.lg : 60,
        bottom: AppSpacing.sm,
      ),
      child: Row(
        mainAxisAlignment: isUser
            ? MainAxisAlignment.end
            : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (!isUser && showAvatar) ...[
            _MiniAvatar(
              initial: avatarInitial ?? '阿',
              colorHex: avatarColor ?? '#7668EE',
            ),
            const SizedBox(width: 8),
          ] else if (!isUser)
            const SizedBox(width: 40),
          Flexible(
            child: Column(
              crossAxisAlignment: isUser
                  ? CrossAxisAlignment.end
                  : CrossAxisAlignment.start,
              children: [
                _buildContent(context, isUser, mock),
                if (message.status == MessageStatus.error)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.error_outline,
                          size: 14,
                          color: context.error,
                        ),
                        const SizedBox(width: 4),
                        GestureDetector(
                          onTap: onRetry,
                          child: Text(
                            '重试',
                            style: TextStyle(
                              fontSize: 12,
                              color: context.error,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildContent(BuildContext context, bool isUser, _MockKind? mock) {
    if (mock != null) {
      switch (mock.type) {
        case _MockType.image:
          return _MockImageMessage(name: mock.args[0], isUser: isUser);
        case _MockType.video:
          return _MockVideoMessage(title: mock.args[0], isUser: isUser);
        case _MockType.audio:
          return _MockAudioMessage(duration: mock.args[0], isUser: isUser);
        case _MockType.emote:
          return _MockEmoteMessage(
            emoji: mock.args[0],
            name: mock.args[1],
            isUser: isUser,
          );
        case _MockType.code:
          return _MockCodeMessage(
            lang: mock.args[0],
            body: mock.args[1],
            isUser: isUser,
          );
      }
    }
    if (message.type == MessageType.file) {
      return _FileMessage(
        fileName: message.fileName ?? '',
        fileSizeKB: message.fileSizeKB ?? 0,
        isUser: isUser,
      );
    }
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * (isUser ? 0.78 : 0.82),
      ),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        color: isUser ? context.accentSoft : context.surfacePrimary,
        borderRadius: BorderRadius.only(
          topLeft: const Radius.circular(16),
          topRight: const Radius.circular(16),
          bottomLeft: Radius.circular(isUser ? 16 : 4),
          bottomRight: Radius.circular(isUser ? 4 : 16),
        ),
        border: isUser
            ? null
            : Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Text(
        message.content,
        style: AppTypography.bodySmall(context).copyWith(
          color: isUser ? context.accentPressed : context.textPrimary,
          height: 1.45,
        ),
      ),
    );
  }
}

class _MiniAvatar extends StatelessWidget {
  final String initial;
  final String colorHex;
  const _MiniAvatar({required this.initial, required this.colorHex});

  @override
  Widget build(BuildContext context) {
    final color = Color(
      int.parse('FF${colorHex.replaceAll('#', '')}', radix: 16),
    );
    return Container(
      width: 32,
      height: 32,
      decoration: BoxDecoration(color: color, shape: BoxShape.circle),
      child: Center(
        child: Text(
          initial,
          style: const TextStyle(
            color: Colors.white,
            fontSize: 14,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }
}

Color _parseHex(String hex) {
  final cleaned = hex.replaceAll('#', '');
  return Color(int.parse('FF$cleaned', radix: 16));
}

class _FileMessage extends StatelessWidget {
  final String fileName;
  final int fileSizeKB;
  final bool isUser;

  const _FileMessage({
    required this.fileName,
    required this.fileSizeKB,
    required this.isUser,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.7,
      ),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isUser ? context.accentSoft : context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: isUser
            ? null
            : Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(
              Icons.description_outlined,
              color: context.accentPrimary,
              size: 22,
            ),
          ),
          const SizedBox(width: 12),
          Flexible(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  fileName,
                  style: AppTypography.bodySmall(context),
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  '${(fileSizeKB / 1024).toStringAsFixed(1)} MB',
                  style: AppTypography.label(context),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _MockImageMessage extends StatelessWidget {
  final String name;
  final bool isUser;
  const _MockImageMessage({required this.name, required this.isUser});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => _preview(context),
      child: Container(
        constraints: BoxConstraints(
          maxWidth: MediaQuery.sizeOf(context).width * 0.6,
        ),
        decoration: BoxDecoration(
          color: _parseHex('#7668EE'),
          borderRadius: AppRadius.brMedium,
          border: isUser
              ? null
              : Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              height: 140,
              decoration: BoxDecoration(
                color: _parseHex('#7668EE'),
                borderRadius: const BorderRadius.vertical(
                  top: Radius.circular(16),
                ),
              ),
              child: Center(
                child: Icon(
                  Icons.image_outlined,
                  size: 40,
                  color: Colors.white.withValues(alpha: 0.8),
                ),
              ),
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
              child: Text(
                name,
                style: const TextStyle(fontSize: 12, color: Colors.white),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _preview(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => GestureDetector(
        onTap: () => Navigator.pop(ctx),
        child: Material(
          color: Colors.black54,
          child: Center(
            child: Container(
              width: MediaQuery.sizeOf(context).width * 0.8,
              height: MediaQuery.sizeOf(context).width * 0.8,
              decoration: BoxDecoration(
                color: _parseHex('#7668EE'),
                borderRadius: AppRadius.brMedium,
              ),
              child: Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      Icons.image_outlined,
                      size: 64,
                      color: Colors.white.withValues(alpha: 0.8),
                    ),
                    const SizedBox(height: 12),
                    Text(
                      name,
                      style: const TextStyle(color: Colors.white, fontSize: 16),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _MockVideoMessage extends StatelessWidget {
  final String title;
  final bool isUser;
  const _MockVideoMessage({required this.title, required this.isUser});

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.6,
      ),
      decoration: BoxDecoration(
        color: const Color(0xFF1E1E2E),
        borderRadius: AppRadius.brMedium,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            height: 120,
            decoration: BoxDecoration(
              color: const Color(0xFF1E1E2E),
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(16),
              ),
            ),
            child: Center(
              child: Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.2),
                  shape: BoxShape.circle,
                ),
                child: const Icon(
                  Icons.play_arrow,
                  color: Colors.white,
                  size: 28,
                ),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
            child: Row(
              children: [
                Icon(
                  Icons.videocam_outlined,
                  size: 14,
                  color: Colors.white.withValues(alpha: 0.7),
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    title,
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.white.withValues(alpha: 0.9),
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _MockAudioMessage extends StatelessWidget {
  final String duration;
  final bool isUser;
  const _MockAudioMessage({required this.duration, required this.isUser});

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.62,
      ),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: isUser ? context.accentSoft : context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: isUser
            ? null
            : Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: context.accentPrimary,
              shape: BoxShape.circle,
            ),
            child: const Icon(Icons.play_arrow, color: Colors.white, size: 20),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: List.generate(
                    18,
                    (i) => Container(
                      margin: const EdgeInsets.only(right: 2),
                      width: 2,
                      height: 6 + (i % 4) * 5.0,
                      color: context.accentPrimary.withValues(alpha: 0.5),
                    ),
                  ),
                ),
                const SizedBox(height: 4),
                Text(duration, style: AppTypography.label(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _MockEmoteMessage extends StatelessWidget {
  final String emoji;
  final String name;
  final bool isUser;
  const _MockEmoteMessage({
    required this.emoji,
    required this.name,
    required this.isUser,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: isUser
          ? CrossAxisAlignment.end
          : CrossAxisAlignment.start,
      children: [
        Container(
          padding: const EdgeInsets.all(8),
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: AppRadius.brMedium,
          ),
          child: Text(emoji, style: const TextStyle(fontSize: 48)),
        ),
        if (name.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 4),
            child: Text(name, style: AppTypography.label(context)),
          ),
      ],
    );
  }
}

class _MockCodeMessage extends StatelessWidget {
  final String lang;
  final String body;
  final bool isUser;
  const _MockCodeMessage({
    required this.lang,
    required this.body,
    required this.isUser,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.82,
      ),
      decoration: BoxDecoration(
        color: const Color(0xFF282C34),
        borderRadius: AppRadius.brMedium,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            child: Row(
              children: [
                Icon(
                  Icons.code,
                  size: 14,
                  color: Colors.white.withValues(alpha: 0.6),
                ),
                const SizedBox(width: 6),
                Text(
                  lang,
                  style: TextStyle(
                    fontSize: 12,
                    color: Colors.white.withValues(alpha: 0.7),
                  ),
                ),
              ],
            ),
          ),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
            child: SelectableText(
              body,
              style: const TextStyle(
                fontFamily: 'monospace',
                fontSize: 13,
                color: Color(0xFFABB2BF),
                height: 1.5,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _AgentTaskMessage extends StatelessWidget {
  final ChatMessage message;
  final VoidCallback? onAgentTaskTap;
  final VoidCallback? onPauseAgentTask;
  final VoidCallback? onResumeAgentTask;
  final String statusLabel;

  const _AgentTaskMessage({
    required this.message,
    this.onAgentTaskTap,
    this.onPauseAgentTask,
    this.onResumeAgentTask,
    required this.statusLabel,
  });

  BadgeType get _badgeType {
    switch (statusLabel) {
      case '运行中':
        return BadgeType.accent;
      case '已暂停':
        return BadgeType.neutral;
      case '已完成':
        return BadgeType.success;
      case '已失败':
        return BadgeType.error;
      default:
        return BadgeType.accent;
    }
  }

  @override
  Widget build(BuildContext context) {
    final isRunning = statusLabel == '运行中';
    final isPaused = statusLabel == '已暂停';
    return Padding(
      padding: const EdgeInsets.only(
        left: AppSpacing.lg,
        right: 60,
        bottom: AppSpacing.sm,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(width: 40),
          Flexible(
            child: Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.borderPrimary, width: 0.5),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        Icons.auto_awesome,
                        size: 16,
                        color: context.accentPrimary,
                      ),
                      const SizedBox(width: 6),
                      Expanded(
                        child: Text(
                          '正在执行：${message.agentTaskTitle ?? ''}',
                          style: AppTypography.cardTitle(
                            context,
                          ).copyWith(fontSize: 14),
                        ),
                      ),
                      AmitiaStatusBadge(label: statusLabel, type: _badgeType),
                    ],
                  ),
                  const SizedBox(height: 12),
                  ...(message.agentTaskSteps ?? []).map(
                    (step) => Padding(
                      padding: const EdgeInsets.only(bottom: 6),
                      child: Row(
                        children: [
                          Container(
                            width: 6,
                            height: 6,
                            decoration: BoxDecoration(
                              color: context.accentPrimary,
                              shape: BoxShape.circle,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Text(step, style: AppTypography.caption(context)),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 10),
                  AmitiaProgressBar(
                    progress: (message.agentTaskProgress ?? 0) / 100,
                  ),
                  const SizedBox(height: 8),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        '进度 ${message.agentTaskProgress ?? 0}%',
                        style: AppTypography.label(context),
                      ),
                      Text(
                        '已用时 ${message.agentTaskElapsed ?? '00:00'}',
                        style: AppTypography.label(context),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      GestureDetector(
                        onTap: onAgentTaskTap,
                        child: Text(
                          '查看详情',
                          style: TextStyle(
                            fontSize: 13,
                            color: context.accentPrimary,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                      const Spacer(),
                      if (isRunning)
                        GestureDetector(
                          onTap: onPauseAgentTask,
                          child: Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 6,
                            ),
                            decoration: BoxDecoration(
                              color: context.accentSoft,
                              borderRadius: AppRadius.brTag,
                            ),
                            child: Text(
                              '暂停',
                              style: TextStyle(
                                fontSize: 12,
                                color: context.accentPrimary,
                              ),
                            ),
                          ),
                        )
                      else if (isPaused)
                        GestureDetector(
                          onTap: onResumeAgentTask,
                          child: Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 6,
                            ),
                            decoration: BoxDecoration(
                              color: context.accentSoft,
                              borderRadius: AppRadius.brTag,
                            ),
                            child: Text(
                              '继续',
                              style: TextStyle(
                                fontSize: 12,
                                color: context.accentPrimary,
                              ),
                            ),
                          ),
                        ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ToolCallMessage extends StatelessWidget {
  final ChatMessage message;
  const _ToolCallMessage({required this.message});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(
        left: AppSpacing.lg,
        right: 60,
        bottom: AppSpacing.sm,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(width: 40),
          Flexible(
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.build_outlined,
                    size: 16,
                    color: context.textTertiary,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: RichText(
                      text: TextSpan(
                        style: AppTypography.caption(context),
                        children: [
                          TextSpan(
                            text: '${message.toolName ?? '工具'}: ',
                            style: TextStyle(
                              color: context.accentPrimary,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                          TextSpan(text: message.toolResult ?? ''),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

const List<(String, String)> _mockImages = [
  ('风景照', '#7668EE'),
  ('截图', '#52B788'),
  ('头像', '#E9A23B'),
  ('壁纸', '#6C8FEA'),
  ('相册', '#E76F51'),
  ('照片', '#9B5DE5'),
  ('图标', '#00BBF9'),
  ('插画', '#F15BB5'),
];

const List<(String, int)> _mockFiles = [
  ('产品需求文档.pdf', 2048),
  ('周报模板.docx', 512),
  ('数据统计.xlsx', 1024),
  ('设计稿.png', 4096),
  ('笔记.md', 64),
  ('演示.pptx', 3072),
];

const List<(String, List<String>)> _mockEmotes = [
  ('常用', ['😀', '😂', '🥰', '😎', '🤔', '😴', '👍', '❤️', '🔥', '🎉']),
  ('阿米娅', ['😊', '🤗', '✨', '🌟', '💫', '🌸', '🌈', '☕']),
  ('动物', ['🐶', '🐱', '🐰', '🦊', '🐼', '🐨', '🐯', '🐸']),
];

const List<String> _codeLanguages = [
  'Dart',
  'Python',
  'JavaScript',
  'Go',
  'SQL',
  'JSON',
  'Shell',
];

class AmitiaChatInput extends StatefulWidget {
  final ValueChanged<String> onSend;
  final bool isAgentMode;
  final ValueChanged<bool>? onAgentModeChanged;
  final void Function(String fileName, int sizeKB)? onSendFile;
  final ValueChanged<String>? onSendImage;
  final void Function(String lang, String code)? onSendCode;
  final ValueChanged<String>? onSendVoice;
  final void Function(String emoji, String name)? onSendEmote;

  const AmitiaChatInput({
    super.key,
    required this.onSend,
    this.isAgentMode = false,
    this.onAgentModeChanged,
    this.onSendFile,
    this.onSendImage,
    this.onSendCode,
    this.onSendVoice,
    this.onSendEmote,
  });

  @override
  State<AmitiaChatInput> createState() => _AmitiaChatInputState();
}

class _AmitiaChatInputState extends State<AmitiaChatInput> {
  final _controller = TextEditingController();
  final _inputFocusNode = FocusNode();
  bool _hasText = false;
  bool _isInputFocused = false;

  @override
  void initState() {
    super.initState();
    _inputFocusNode.addListener(_syncInputFocus);
  }

  void _syncInputFocus() {
    setState(() => _isInputFocused = _inputFocusNode.hasFocus);
  }

  @override
  void dispose() {
    _inputFocusNode.removeListener(_syncInputFocus);
    _inputFocusNode.dispose();
    _controller.dispose();
    super.dispose();
  }

  void _send() {
    final text = _controller.text.trim();
    if (text.isEmpty) return;
    widget.onSend(text);
    _controller.clear();
    setState(() => _hasText = false);
  }

  void _showFileSheet() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: context.borderPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 20),
                Text('选择文件', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 12),
                ListTile(
                  leading: _sheetIcon(
                    context,
                    Icons.folder_outlined,
                    context.accentPrimary,
                  ),
                  title: const Text('选择文件'),
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    final f = _mockFiles.first;
                    widget.onSendFile?.call(f.$1, f.$2);
                  },
                ),
                ListTile(
                  leading: _sheetIcon(
                    context,
                    Icons.history,
                    context.accentPrimary,
                  ),
                  title: const Text('从工作区选择'),
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    final f = _mockFiles[2];
                    widget.onSendFile?.call(f.$1, f.$2);
                  },
                ),
                const Divider(height: 1),
                const SizedBox(height: 8),
                Text(
                  '最近文件',
                  style: AppTypography.label(
                    context,
                  ).copyWith(fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 4),
                ..._mockFiles.map(
                  (f) => ListTile(
                    leading: _sheetIcon(
                      context,
                      Icons.description_outlined,
                      context.textTertiary,
                    ),
                    title: Text(f.$1, style: AppTypography.bodySmall(context)),
                    trailing: Text(
                      '${(f.$2 / 1024).toStringAsFixed(1)} MB',
                      style: AppTypography.label(context),
                    ),
                    onTap: () {
                      Navigator.pop(sheetCtx);
                      widget.onSendFile?.call(f.$1, f.$2);
                    },
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _showImagePicker() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: context.borderPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 20),
                Text('选择图片', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                GridView.builder(
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 4,
                    mainAxisSpacing: 10,
                    crossAxisSpacing: 10,
                    childAspectRatio: 1,
                  ),
                  itemCount: _mockImages.length,
                  itemBuilder: (ctx, i) {
                    final img = _mockImages[i];
                    return GestureDetector(
                      onTap: () {
                        Navigator.pop(sheetCtx);
                        widget.onSendImage?.call(img.$1);
                      },
                      child: Container(
                        decoration: BoxDecoration(
                          color: _parseHex(img.$2),
                          borderRadius: AppRadius.brSmall,
                        ),
                        child: Column(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Icon(
                              Icons.image_outlined,
                              color: Colors.white.withValues(alpha: 0.85),
                              size: 22,
                            ),
                            const SizedBox(height: 4),
                            Text(
                              img.$1,
                              style: const TextStyle(
                                color: Colors.white,
                                fontSize: 11,
                              ),
                            ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _showCodeDialog() {
    String lang = _codeLanguages.first;
    final codeCtrl = TextEditingController();
    showDialog(
      context: context,
      builder: (ctx) {
        return StatefulBuilder(
          builder: (ctx, setSheetState) {
            return AlertDialog(
              backgroundColor: context.surfacePrimary,
              shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
              title: Text('插入代码', style: AppTypography.cardTitle(ctx)),
              content: SizedBox(
                width: MediaQuery.sizeOf(context).width * 0.9,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '语言',
                      style: AppTypography.label(
                        ctx,
                      ).copyWith(fontWeight: FontWeight.w600),
                    ),
                    const SizedBox(height: 6),
                    DropdownButtonFormField<String>(
                      value: lang,
                      decoration: InputDecoration(
                        isDense: true,
                        border: OutlineInputBorder(
                          borderRadius: AppRadius.brSmall,
                        ),
                      ),
                      items: _codeLanguages
                          .map(
                            (l) => DropdownMenuItem(
                              value: l,
                              child: Text(
                                l,
                                style: AppTypography.bodySmall(ctx),
                              ),
                            ),
                          )
                          .toList(),
                      onChanged: (v) =>
                          setSheetState(() => lang = v ?? _codeLanguages.first),
                    ),
                    const SizedBox(height: 14),
                    Text(
                      '代码',
                      style: AppTypography.label(
                        ctx,
                      ).copyWith(fontWeight: FontWeight.w600),
                    ),
                    const SizedBox(height: 6),
                    TextField(
                      controller: codeCtrl,
                      maxLines: 6,
                      style: const TextStyle(
                        fontFamily: 'monospace',
                        fontSize: 13,
                      ),
                      decoration: InputDecoration(
                        hintText: '输入代码……',
                        hintStyle: TextStyle(color: context.textTertiary),
                        isDense: true,
                        border: OutlineInputBorder(
                          borderRadius: AppRadius.brSmall,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(ctx),
                  child: Text(
                    '取消',
                    style: TextStyle(color: context.textSecondary),
                  ),
                ),
                AmitiaButton(
                  label: '插入',
                  height: 40,
                  onPressed: () {
                    final code = codeCtrl.text.trim();
                    if (code.isEmpty) return;
                    Navigator.pop(ctx);
                    widget.onSendCode?.call(lang, code);
                  },
                ),
              ],
            );
          },
        );
      },
    );
  }

  void _showVoiceSheet() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      isScrollControlled: true,
      builder: (sheetCtx) {
        return _VoiceRecorderSheet(
          onSend: (duration) {
            Navigator.pop(sheetCtx);
            widget.onSendVoice?.call(duration);
          },
        );
      },
    );
  }

  void _showEmotePicker() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) {
        return _EmotePicker(
          onSend: (emoji, name) {
            Navigator.pop(sheetCtx);
            widget.onSendEmote?.call(emoji, name);
          },
        );
      },
    );
  }

  Widget _sheetIcon(BuildContext context, IconData icon, Color color) {
    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: AppRadius.brSmall,
      ),
      child: Icon(icon, size: 20, color: color),
    );
  }

  void _showComposerTools() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      builder: (sheetContext) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 36,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              const SizedBox(height: 12),
              _ComposerTool(
                icon: Icons.photo_library_outlined,
                label: '添加图片',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showImagePicker();
                },
              ),
              _ComposerTool(
                icon: Icons.attach_file_outlined,
                label: '添加文件',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showFileSheet();
                },
              ),
              _ComposerTool(
                icon: Icons.code,
                label: '插入代码',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showCodeDialog();
                },
              ),
              _ComposerTool(
                icon: Icons.emoji_emotions_outlined,
                label: '选择表情',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showEmotePicker();
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
        child: AnimatedContainer(
          key: const ValueKey('chat-composer-surface'),
          duration: AppMotion.standard,
          curve: AppMotion.standardCurve,
          constraints: const BoxConstraints(minHeight: 98, maxHeight: 170),
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: BorderRadius.circular(24),
            border: Border.all(
              color: _isInputFocused
                  ? context.accentPrimary
                  : context.borderPrimary,
              width: _isInputFocused ? 1.2 : 0.8,
            ),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                controller: _controller,
                focusNode: _inputFocusNode,
                minLines: 1,
                maxLines: 4,
                textCapitalization: TextCapitalization.sentences,
                onChanged: (value) =>
                    setState(() => _hasText = value.trim().isNotEmpty),
                onSubmitted: (_) => _send(),
                style: AppTypography.bodySmall(context).copyWith(fontSize: 16),
                decoration: InputDecoration(
                  hintText: '有什么可以帮忙的？',
                  hintStyle: TextStyle(
                    color: context.textTertiary,
                    fontSize: 16,
                  ),
                  isDense: true,
                  contentPadding: const EdgeInsets.fromLTRB(16, 14, 16, 8),
                  border: InputBorder.none,
                  enabledBorder: InputBorder.none,
                  focusedBorder: InputBorder.none,
                  disabledBorder: InputBorder.none,
                  errorBorder: InputBorder.none,
                  focusedErrorBorder: InputBorder.none,
                  fillColor: Colors.transparent,
                  focusColor: Colors.transparent,
                  hoverColor: Colors.transparent,
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(8, 0, 8, 8),
                child: Row(
                  children: [
                    IconButton(
                      tooltip: '添加内容',
                      onPressed: _showComposerTools,
                      icon: Container(
                        width: 28,
                        height: 28,
                        decoration: BoxDecoration(
                          color: context.backgroundPrimary,
                          shape: BoxShape.circle,
                          border: Border.all(color: context.borderPrimary),
                        ),
                        child: Icon(
                          Icons.add,
                          size: 19,
                          color: context.textPrimary,
                        ),
                      ),
                    ),
                    _ClaudeStyleAgentChip(
                      isEnabled: widget.isAgentMode,
                      onTap: () =>
                          widget.onAgentModeChanged?.call(!widget.isAgentMode),
                    ),
                    const Spacer(),
                    IconButton(
                      tooltip: '开始语音输入',
                      onPressed: _showVoiceSheet,
                      icon: Icon(
                        Icons.mic_none_outlined,
                        size: 23,
                        color: context.textPrimary,
                      ),
                    ),
                    IconButton(
                      tooltip: '发送消息',
                      onPressed: _hasText ? _send : null,
                      icon: AnimatedContainer(
                        duration: AppMotion.standard,
                        curve: AppMotion.standardCurve,
                        width: 32,
                        height: 32,
                        decoration: BoxDecoration(
                          color: _hasText
                              ? context.accentPrimary
                              : context.borderPrimary,
                          shape: BoxShape.circle,
                        ),
                        child: Icon(
                          Icons.arrow_upward_rounded,
                          size: 19,
                          color: _hasText
                              ? context.surfacePrimary
                              : context.textTertiary,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ClaudeStyleAgentChip extends StatelessWidget {
  final bool isEnabled;
  final VoidCallback onTap;

  const _ClaudeStyleAgentChip({required this.isEnabled, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: onTap,
        child: Container(
          height: 32,
          padding: const EdgeInsets.symmetric(horizontal: 10),
          decoration: BoxDecoration(
            color: isEnabled ? context.accentSoft : context.backgroundPrimary,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(
              color: isEnabled ? context.accentPrimary : context.borderPrimary,
            ),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.hub_outlined,
                size: 15,
                color: isEnabled
                    ? context.accentPrimary
                    : context.textSecondary,
              ),
              const SizedBox(width: 5),
              Text(
                'Agent',
                style: AppTypography.label(context).copyWith(
                  color: isEnabled
                      ? context.accentPrimary
                      : context.textSecondary,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ComposerTool extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  const _ComposerTool({
    required this.icon,
    required this.label,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: AppRadius.brSmall,
        onTap: onTap,
        child: SizedBox(
          height: 52,
          child: Row(
            children: [
              Icon(icon, size: 22, color: context.textPrimary),
              const SizedBox(width: 16),
              Text(label, style: AppTypography.body(context)),
            ],
          ),
        ),
      ),
    );
  }
}

enum _VoiceState { idle, recording, paused, done }

class _VoiceRecorderSheet extends StatefulWidget {
  final ValueChanged<String> onSend;
  const _VoiceRecorderSheet({required this.onSend});

  @override
  State<_VoiceRecorderSheet> createState() => _VoiceRecorderSheetState();
}

class _VoiceRecorderSheetState extends State<_VoiceRecorderSheet> {
  _VoiceState _state = _VoiceState.idle;
  int _seconds = 0;
  Timer? _timer;

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  void _start() {
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      setState(() => _seconds++);
    });
    setState(() => _state = _VoiceState.recording);
  }

  void _pause() {
    _timer?.cancel();
    _timer = null;
    setState(() => _state = _VoiceState.paused);
  }

  void _resume() {
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      setState(() => _seconds++);
    });
    setState(() => _state = _VoiceState.recording);
  }

  void _finish() {
    _timer?.cancel();
    _timer = null;
    setState(() => _state = _VoiceState.done);
  }

  void _reset() {
    _timer?.cancel();
    _timer = null;
    setState(() {
      _state = _VoiceState.idle;
      _seconds = 0;
    });
  }

  String get _duration {
    final m = (_seconds ~/ 60).toString().padLeft(2, '0');
    final s = (_seconds % 60).toString().padLeft(2, '0');
    return '$m:$s';
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 8),
            Center(
              child: Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: 20),
            Text('语音录制', style: AppTypography.pageTitle(context)),
            const SizedBox(height: 24),
            Container(
              width: 96,
              height: 96,
              decoration: BoxDecoration(
                color: _state == _VoiceState.recording
                    ? context.error.withValues(alpha: 0.12)
                    : context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Icon(
                  _state == _VoiceState.recording ? Icons.mic : Icons.mic_none,
                  size: 44,
                  color: _state == _VoiceState.recording
                      ? context.error
                      : context.accentPrimary,
                ),
              ),
            ),
            const SizedBox(height: 16),
            Text(
              _duration,
              style: AppTypography.pageLargeTitle(
                context,
              ).copyWith(fontFeatures: const [FontFeature.tabularFigures()]),
            ),
            const SizedBox(height: 8),
            Text(
              _state == _VoiceState.idle
                  ? '点击开始录音'
                  : _state == _VoiceState.recording
                  ? '正在录音……'
                  : _state == _VoiceState.paused
                  ? '已暂停'
                  : '录制完成',
              style: AppTypography.caption(context),
            ),
            const SizedBox(height: 24),
            _buildControls(),
          ],
        ),
      ),
    );
  }

  Widget _buildControls() {
    switch (_state) {
      case _VoiceState.idle:
        return AmitiaButton(
          label: '开始录音',
          icon: Icons.fiber_manual_record,
          isFullWidth: true,
          onPressed: _start,
        );
      case _VoiceState.recording:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '暂停',
                icon: Icons.pause,
                isSecondary: true,
                isFullWidth: true,
                onPressed: _pause,
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: AmitiaButton(
                label: '完成',
                icon: Icons.stop,
                isFullWidth: true,
                onPressed: _finish,
              ),
            ),
          ],
        );
      case _VoiceState.paused:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '继续',
                icon: Icons.play_arrow,
                isSecondary: true,
                isFullWidth: true,
                onPressed: _resume,
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: AmitiaButton(
                label: '完成',
                icon: Icons.stop,
                isFullWidth: true,
                onPressed: _finish,
              ),
            ),
          ],
        );
      case _VoiceState.done:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '重录',
                icon: Icons.refresh,
                isSecondary: true,
                isFullWidth: true,
                onPressed: _reset,
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: AmitiaButton(
                label: '发送',
                icon: Icons.send,
                isFullWidth: true,
                onPressed: () => widget.onSend(_duration),
              ),
            ),
          ],
        );
    }
  }
}

class _EmotePicker extends StatefulWidget {
  final void Function(String emoji, String name) onSend;
  const _EmotePicker({required this.onSend});

  @override
  State<_EmotePicker> createState() => _EmotePickerState();
}

class _EmotePickerState extends State<_EmotePicker> {
  int _group = 0;

  @override
  Widget build(BuildContext context) {
    final group = _mockEmotes[_group];
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 0, 12, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 8),
            Center(
              child: Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: 16),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8),
              child: Row(
                children: List.generate(_mockEmotes.length, (i) {
                  final isSelected = i == _group;
                  return GestureDetector(
                    onTap: () => setState(() => _group = i),
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 14,
                        vertical: 6,
                      ),
                      margin: const EdgeInsets.only(right: 8),
                      decoration: BoxDecoration(
                        color: isSelected
                            ? context.accentSoft
                            : Colors.transparent,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(
                        _mockEmotes[i].$1,
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: isSelected
                              ? FontWeight.w600
                              : FontWeight.w400,
                          color: isSelected
                              ? context.accentPrimary
                              : context.textSecondary,
                        ),
                      ),
                    ),
                  );
                }),
              ),
            ),
            const Divider(height: 20),
            SizedBox(
              height: 220,
              child: GridView.builder(
                padding: const EdgeInsets.symmetric(horizontal: 4),
                gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: 5,
                  mainAxisSpacing: 4,
                  crossAxisSpacing: 4,
                  childAspectRatio: 1,
                ),
                itemCount: group.$2.length,
                itemBuilder: (ctx, i) {
                  final emoji = group.$2[i];
                  return GestureDetector(
                    onTap: () => widget.onSend(emoji, group.$1),
                    child: Container(
                      decoration: BoxDecoration(
                        color: context.surfaceSecondary,
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: Center(
                        child: Text(
                          emoji,
                          style: const TextStyle(fontSize: 28),
                        ),
                      ),
                    ),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class AmitiaPermissionSheet extends StatelessWidget {
  final String taskTitle;
  final List<String> permissions;
  final VoidCallback onAllowOnce;
  final VoidCallback onAllowAlways;
  final VoidCallback onDeny;

  const AmitiaPermissionSheet({
    super.key,
    required this.taskTitle,
    required this.permissions,
    required this.onAllowOnce,
    required this.onAllowAlways,
    required this.onDeny,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Amitia 想执行以下操作', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 4),
          Text(taskTitle, style: AppTypography.caption(context)),
          const SizedBox(height: 20),
          ...permissions.map(
            (p) => Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Row(
                children: [
                  Icon(
                    Icons.check_circle_outline,
                    size: 20,
                    color: context.accentPrimary,
                  ),
                  const SizedBox(width: 12),
                  Expanded(child: Text(p, style: AppTypography.body(context))),
                ],
              ),
            ),
          ),
          const SizedBox(height: 8),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: context.warning.withValues(alpha: 0.08),
              borderRadius: AppRadius.brSmall,
            ),
            child: Row(
              children: [
                Icon(Icons.shield_outlined, size: 18, color: context.warning),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    '风险等级：中等 · 操作范围：本地文件',
                    style: AppTypography.label(
                      context,
                    ).copyWith(color: context.warning),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
          AmitiaButton(
            label: '此次允许',
            isFullWidth: true,
            onPressed: onAllowOnce,
          ),
          const SizedBox(height: 8),
          AmitiaButton(
            label: '始终允许此工具',
            isFullWidth: true,
            isSecondary: true,
            onPressed: onAllowAlways,
          ),
          const SizedBox(height: 8),
          AmitiaButton(
            label: '拒绝',
            isFullWidth: true,
            isDestructive: true,
            onPressed: onDeny,
          ),
        ],
      ),
    );
  }
}
