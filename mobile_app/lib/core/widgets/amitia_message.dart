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
        padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
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
                _buildContent(context, isUser),
                if (message.status == MessageStatus.error)
                  Padding(
                    padding: EdgeInsets.only(top: 4),
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

  Widget _buildContent(BuildContext context, bool isUser) {
    switch (message.type) {
      case MessageType.image:
        return _ImageMessage(
          name: message.fileName ?? '图片',
          url: message.mediaUrl,
          isUser: isUser,
        );
      case MessageType.video:
        return _VideoMessage(
          title: message.fileName ?? '视频',
          durationMs: message.durationMs ?? 0,
          isUser: isUser,
        );
      case MessageType.audio:
        return _AudioMessage(
          title: message.fileName ?? '语音消息',
          durationMs: message.durationMs ?? 0,
          isUser: isUser,
        );
      case MessageType.emote:
        return _EmoteMessage(
          emoji: message.content,
          name: '',
          isUser: isUser,
        );
      case MessageType.code:
        final parsed = _parseCodeFence(message.content);
        return _CodeMessage(
          lang: parsed.$1,
          body: parsed.$2,
          isUser: isUser,
        );
      case MessageType.file:
        return _FileMessage(
          fileName: message.fileName ?? message.content,
          fileSizeKB: message.fileSizeKB ?? 0,
          isUser: isUser,
        );
      case MessageType.text:
      case MessageType.agentTask:
      case MessageType.toolCall:
      case MessageType.systemNotice:
        break;
    }
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * (isUser ? 0.78 : 0.82),
      ),
      padding: EdgeInsets.symmetric(horizontal: 14, vertical: 10),
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

  (String, String) _parseCodeFence(String content) {
    final trimmed = content.trim();
    if (!trimmed.startsWith('```') || !trimmed.endsWith('```')) {
      return ('text', content);
    }
    final withoutPrefix = trimmed.substring(3);
    final newline = withoutPrefix.indexOf('\n');
    if (newline < 0) return ('text', withoutPrefix.replaceFirst(RegExp(r'```$'), ''));
    final language = withoutPrefix.substring(0, newline).trim();
    final body = withoutPrefix.substring(newline + 1, withoutPrefix.length - 3);
    return (language.isEmpty ? 'text' : language, body);
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

class _ImageMessage extends StatelessWidget {
  final String name;
  final String? url;
  final bool isUser;
  const _ImageMessage({required this.name, this.url, required this.isUser});

  @override
  Widget build(BuildContext context) {
    final hasUrl = url != null && url!.isNotEmpty;
    return GestureDetector(
      onTap: hasUrl ? () => _preview(context) : null,
      child: Container(
        constraints: BoxConstraints(
          maxWidth: MediaQuery.sizeOf(context).width * 0.6,
        ),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brMedium,
          border: isUser
              ? null
              : Border.all(color: context.borderPrimary, width: 0.5),
        ),
        clipBehavior: Clip.antiAlias,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              height: 160,
              width: double.infinity,
              child: hasUrl
                  ? Image.network(
                      url!,
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => _placeholder(context),
                    )
                  : _placeholder(context),
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
              child: Text(
                name,
                style: AppTypography.label(context),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _placeholder(BuildContext context) => Container(
        color: context.accentSoft,
        alignment: Alignment.center,
        child: Icon(
          Icons.image_outlined,
          size: 40,
          color: context.accentPrimary,
        ),
      );

  void _preview(BuildContext context) {
    final value = url;
    if (value == null || value.isEmpty) return;
    showDialog(
      context: context,
      builder: (ctx) => GestureDetector(
        onTap: () => Navigator.pop(ctx),
        child: Material(
          color: Colors.black87,
          child: SafeArea(
            child: Center(
              child: InteractiveViewer(
                child: Image.network(
                  value,
                  fit: BoxFit.contain,
                  errorBuilder: (_, __, ___) => const Icon(
                    Icons.broken_image_outlined,
                    color: Colors.white,
                    size: 64,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _VideoMessage extends StatelessWidget {
  final String title;
  final int durationMs;
  final bool isUser;
  const _VideoMessage({
    required this.title,
    required this.durationMs,
    required this.isUser,
  });

  String get _duration {
    if (durationMs <= 0) return '视频';
    final total = durationMs ~/ 1000;
    final min = total ~/ 60;
    final sec = (total % 60).toString().padLeft(2, '0');
    return '$min:$sec';
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.6,
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
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(
              Icons.videocam_outlined,
              color: context.accentPrimary,
            ),
          ),
          const SizedBox(width: 10),
          Flexible(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: AppTypography.bodySmall(context),
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(_duration, style: AppTypography.label(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _AudioMessage extends StatelessWidget {
  final String title;
  final int durationMs;
  final bool isUser;
  const _AudioMessage({
    required this.title,
    required this.durationMs,
    required this.isUser,
  });

  String get _duration {
    if (durationMs <= 0) return '语音消息';
    final total = durationMs ~/ 1000;
    final min = total ~/ 60;
    final sec = (total % 60).toString().padLeft(2, '0');
    return '$min:$sec';
  }

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
              color: context.accentSoft,
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.graphic_eq_rounded,
              color: context.accentPrimary,
              size: 20,
            ),
          ),
          const SizedBox(width: 10),
          Flexible(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: AppTypography.bodySmall(context),
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(_duration, style: AppTypography.label(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _EmoteMessage extends StatelessWidget {
  final String emoji;
  final String name;
  final bool isUser;
  const _EmoteMessage({
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
            padding: EdgeInsets.only(top: 4),
            child: Text(name, style: AppTypography.label(context)),
          ),
      ],
    );
  }
}

class _CodeMessage extends StatelessWidget {
  final String lang;
  final String body;
  final bool isUser;
  const _CodeMessage({
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
            padding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
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
            padding: EdgeInsets.symmetric(horizontal: 12, vertical: 10),
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
      padding: EdgeInsets.only(
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
                      padding: EdgeInsets.only(bottom: 6),
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
                            padding: EdgeInsets.symmetric(
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
                            padding: EdgeInsets.symmetric(
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
      padding: EdgeInsets.only(
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
              padding: EdgeInsets.symmetric(horizontal: 12, vertical: 10),
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

const List<(String, List<String>)> _emojiGroups = [
  ('常用', ['😀', '😂', '🥰', '😎', '🤔', '😴', '👍', '❤️', '🔥', '🎉']),
  ('Amitia', ['😊', '🤗', '✨', '🌟', '💫', '🌸', '🌈', '☕']),
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
  final FutureOr<void> Function()? onPickFile;
  final FutureOr<void> Function(bool camera)? onPickImage;
  final FutureOr<void> Function(bool camera)? onPickVideo;
  final FutureOr<void> Function()? onPickAudio;
  final void Function(String lang, String code)? onSendCode;
  final void Function(String emoji, String name)? onSendEmote;

  const AmitiaChatInput({
    super.key,
    required this.onSend,
    this.isAgentMode = false,
    this.onAgentModeChanged,
    this.onPickFile,
    this.onPickImage,
    this.onPickVideo,
    this.onPickAudio,
    this.onSendCode,
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
      builder: (sheetCtx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('添加文件', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 12),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.folder_open_outlined,
                  context.accentPrimary,
                ),
                title: const Text('从本机选择'),
                subtitle: const Text('上传后作为真实 Artifact 发送'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickFile?.call();
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showImagePicker() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('添加图片', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 12),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.photo_library_outlined,
                  context.accentPrimary,
                ),
                title: const Text('从相册选择'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickImage?.call(false);
                },
              ),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.photo_camera_outlined,
                  context.accentPrimary,
                ),
                title: const Text('拍照'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickImage?.call(true);
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showVideoPicker() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('添加视频', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 12),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.video_library_outlined,
                  context.accentPrimary,
                ),
                title: const Text('从相册选择'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickVideo?.call(false);
                },
              ),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.videocam_outlined,
                  context.accentPrimary,
                ),
                title: const Text('拍摄视频'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickVideo?.call(true);
                },
              ),
            ],
          ),
        ),
      ),
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
    widget.onPickAudio?.call();
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
          padding: EdgeInsets.fromLTRB(16, 8, 16, 20),
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
                icon: Icons.video_library_outlined,
                label: '添加视频',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showVideoPicker();
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
        padding: EdgeInsets.fromLTRB(12, 8, 12, 12),
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
                  contentPadding: EdgeInsets.fromLTRB(16, 14, 16, 8),
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
                padding: EdgeInsets.fromLTRB(8, 0, 8, 8),
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
          padding: EdgeInsets.symmetric(horizontal: 10),
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
    final group = _emojiGroups[_group];
    return SafeArea(
      child: Padding(
        padding: EdgeInsets.fromLTRB(12, 0, 12, 20),
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
              padding: EdgeInsets.symmetric(horizontal: 8),
              child: Row(
                children: List.generate(_emojiGroups.length, (i) {
                  final isSelected = i == _group;
                  return GestureDetector(
                    onTap: () => setState(() => _group = i),
                    child: Container(
                      padding: EdgeInsets.symmetric(
                        horizontal: 14,
                        vertical: 6,
                      ),
                      margin: EdgeInsets.only(right: 8),
                      decoration: BoxDecoration(
                        color: isSelected
                            ? context.accentSoft
                            : Colors.transparent,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(
                        _emojiGroups[i].$1,
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
                padding: EdgeInsets.symmetric(horizontal: 4),
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
      padding: EdgeInsets.fromLTRB(20, 0, 20, 34),
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
              padding: EdgeInsets.only(bottom: 12),
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
